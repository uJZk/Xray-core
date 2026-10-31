# 自动同步上游正式版并发布镜像

`.github/workflows/sync-upstream.yml` 每小时第 17 分钟检查 `XTLS/Xray-core` 的最新正式版 Release，也可在 Actions 手动运行。定时工作流必须存在于仓库默认分支才会运行，实际调度时间可能延迟。

流程：

1. 查询 GitHub latest release（排除 draft 和 prerelease）。
2. 若该版本的提交已包含在默认分支，直接跳过。
3. 推送 `sync-upstream/<版本>` 分支并创建 PR；已有开放 PR 时复用。
4. GitHub 确认没有冲突后，通过 merge commit 合并，保留 fork 修改。有冲突或暂时无法确定时留下 PR，下一轮重新检查。
5. 合并成功后调用 `docker.yml`，构建准确的合并提交，并发布版本标签（去掉开头 v）和 `latest` 到 `ghcr.io/<owner>/<repo>`。

启用：将工作流和 `.github/scripts/sync-upstream.cjs` 一起提交到默认分支。在仓库 Settings → Actions → General 开启 Actions，并允许 GitHub Actions 创建和批准 Pull Requests（Allow GitHub Actions to create and approve pull requests）。使用仓库自带 `GITHUB_TOKEN`，无需新增 PAT。仓库还需允许 merge commit。

分支保护和组织策略仍然生效；要求审核或必需检查时，GitHub 可能拒绝自动合并，工作流会报错，不会发布镜像。`GITHUB_TOKEN` 创建 PR 和合并不会触发普通 PR/push 工作流；本流程不依赖这些事件，而是显式调用容器工作流，也不额外执行完整项目测试。

有冲突的 PR 需要人工处理。若人工直接合并，定时任务随后会因版本已包含而跳过；此时请手动运行容器工作流发布。自动合并后如果镜像发布失败，可在该次 Actions 运行中选择 Re-run failed jobs，保留 sync 作业输出的准确提交；新建一次同步运行会跳过已合入版本。

当前工作流只查询最新正式版，不逐个补同步历史 Release。首次启用时也会同步尚未包含的当前最新正式版。
