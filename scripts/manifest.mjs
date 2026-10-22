import fs from 'node:fs';
import {execFileSync} from 'node:child_process';
import {createHash} from 'node:crypto';
const files=['BillBook-Windows-x86.exe','BillBook-Windows-x64.exe'].map(name=>({name,sha256:createHash('sha256').update(fs.readFileSync('release/'+name)).digest('hex'),bytes:fs.statSync('release/'+name).size}));
const output={product:'Ledgerleaf',version:'0.1.0',status:'Incomplete development build — not production-ready',built_at:new Date().toISOString(),go:execFileSync(process.env.GO_BIN||'go',['version'],{encoding:'utf8'}).trim(),node:process.version,frontend:JSON.parse(fs.readFileSync('frontend/package.json','utf8')),windows_runtime:'Unverified on all target Windows versions',files};
fs.writeFileSync('release/manifest.json',JSON.stringify(output,null,2)+'\n');
