const vm = require('node:vm');
const fs = require('node:fs');
const assert = require('node:assert/strict');
const source = fs.readFileSync(require('node:path').join(__dirname, 'sync-upstream.cjs'), 'utf8');
async function run({included=false, conflict=false, existing=false, mergeFails=false}={}) {
 const outputs={}, calls=[];
 const pr={number:1,html_url:'https://example/pr/1',mergeable:!conflict};
 const module={exports:{}};
 vm.runInNewContext(source, {module, setTimeout, require:()=>({
  execFileSync:(_cmd,args)=>{calls.push(args[0]);return args[0]==='rev-parse'?'upstream-sha\n':'';},
  spawnSync:()=>({status:included?0:1}),
 })});
 const github={rest:{repos:{getLatestRelease:async()=>({data:{tag_name:'v26.10.31',html_url:'https://example/release'}})},pulls:{
  list:async()=>({data:existing?[pr]:[]}),
  create:async()=>{calls.push('createPR');return {data:pr};},
  get:async()=>({data:pr}),
  merge:async(args)=>{assert.equal(args.sha,'upstream-sha');calls.push('mergePR');return {data:{merged:!mergeFails,sha:'merge-sha'}};},
 }}};
 await module.exports({github,context:{repo:{owner:'fork',repo:'Xray-core'},payload:{repository:{default_branch:'main'}}},core:{setOutput:(k,v)=>outputs[k]=v,info:()=>{},notice:()=>{},warning:()=>{}}});
 return {outputs,calls};
}
(async()=>{
 assert.equal((await run({included:true})).calls.includes('push'),false);
 const conflict=await run({conflict:true});assert.equal(conflict.outputs.merged,'false');assert.equal(conflict.calls.includes('mergePR'),false);
 const merged=await run();assert.deepEqual(merged.outputs,{merged:'true',tag:'v26.10.31',sha:'merge-sha'});
 assert.equal((await run({existing:true})).calls.includes('createPR'),false);
 await assert.rejects(run({mergeFails:true}));
 console.log('PASS: already synced, conflicting, clean merge, existing PR, rejected merge');
})();
