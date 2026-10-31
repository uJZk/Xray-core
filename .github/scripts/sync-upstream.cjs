const {execFileSync, spawnSync} = require('node:child_process');

module.exports = async ({github, context, core}) => {
  const repo = context.repo;
  const upstream = {owner: 'XTLS', repo: 'Xray-core'};
  const base = context.payload.repository.default_branch;
  const git = (...args) => execFileSync('git', args, {encoding: 'utf8'}).trim();
  core.setOutput('merged', 'false');

  // GitHub's latest endpoint excludes drafts and prereleases.
  const {data: release} = await github.rest.repos.getLatestRelease(upstream);
  if (release.draft || release.prerelease) throw new Error('Expected a stable release');
  const tag = release.tag_name;
  // Used in Git refs and Docker tags: reject unexpected characters early.
  if (!/^v?\d+\.\d+\.\d+(?:[.-][A-Za-z0-9_.-]+)?$/.test(tag)) {
    throw new Error(`Unsupported upstream release tag: ${tag}`);
  }
  git('fetch', '--no-tags', 'https://github.com/XTLS/Xray-core.git', `refs/tags/${tag}`);
  const head = git('rev-parse', 'FETCH_HEAD^{commit}');
  const ancestor = spawnSync('git', ['merge-base', '--is-ancestor', head, 'HEAD']);
  if (ancestor.status === 0) {
    core.info(`${tag} is already included in ${base}`);
    return;
  }
  if (ancestor.status !== 1) throw new Error('Unable to check upstream ancestry');

  // A branch at the upstream commit allows GitHub to show conflicts without
  // committing conflict resolutions or overwriting any fork changes.
  const branch = `sync-upstream/${tag}`;
  git('push', 'origin', `${head}:refs/heads/${branch}`);
  const {data: existing} = await github.rest.pulls.list({
    ...repo, state: 'open', base, head: `${repo.owner}:${branch}`,
  });
  let pr = existing[0];
  if (!pr) {
    ({data: pr} = await github.rest.pulls.create({
      ...repo, base, head: branch,
      title: `chore: sync upstream ${tag}`,
      body: `Merge upstream stable release [${tag}](${release.html_url}).\n\nPreserves fork changes through a merge commit. Conflicts require manual resolution. After automatic merge, the sync workflow publishes the merged commit to GHCR with the version and latest tags.`,
    }));
  }
  core.notice(`Upstream sync PR: ${pr.html_url}`);

  // GitHub computes mergeability asynchronously.
  for (let attempt = 0; attempt < 6; attempt++) {
    ({data: pr} = await github.rest.pulls.get({...repo, pull_number: pr.number}));
    if (pr.mergeable !== null) break;
    await new Promise(resolve => setTimeout(resolve, 5000));
  }
  if (pr.mergeable !== true) {
    core.warning(`PR #${pr.number} has conflicts or mergeability is pending; leaving it open.`);
    return;
  }
  // sha prevents merging a changed PR head. GitHub also enforces branch protection.
  const {data: result} = await github.rest.pulls.merge({
    ...repo, pull_number: pr.number, sha: head, merge_method: 'merge',
  });
  if (!result.merged) throw new Error(result.message || 'PR was not merged');
  core.setOutput('tag', tag);
  core.setOutput('sha', result.sha);
  core.setOutput('merged', 'true');
};
