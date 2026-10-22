import test from 'node:test';
import assert from 'node:assert/strict';
import ts from 'typescript';
import vm from 'node:vm';
import fs from 'node:fs';
const context={exports:{}};
vm.runInNewContext(ts.transpileModule(fs.readFileSync(new URL('../src/invoice_image.ts',import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020}}).outputText,context);
const {wrapImageText}=context.exports;
const measure={measureText:text=>({width:Array.from(text).length})};
test('image wrapping preserves complete long tokens, paragraphs and Unicode',()=>{
 const token='LONG'.repeat(1000),unicode='नमस्ते ગુજરાત Café 😀';
 const lines=Array.from(wrapImageText(measure,token+'\n\n'+unicode,25));
 assert.equal(lines.slice(0,160).join(''),token);
 assert.equal(lines[160],'');
 assert.equal(lines.slice(161).join(' '),unicode);
 assert.ok(lines.every(line=>Array.from(line).length<=25));
});
test('image wrapping never hides document text containing markup or long numbers',()=>{
 const text='<script>literal text</script> 12345678901234567890';
 const wrapped=Array.from(wrapImageText(measure,text,8));
 assert.equal(wrapped.join('').replaceAll(' ',''),text.replaceAll(' ',''));
});
