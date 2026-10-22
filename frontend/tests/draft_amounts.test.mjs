import test from 'node:test';
import assert from 'node:assert/strict';
import ts from 'typescript';
import vm from 'node:vm';
import fs from 'node:fs';
function load(name,require=()=>{throw new Error('Unexpected import')}){
 const context={exports:{},require};
 vm.runInNewContext(ts.transpileModule(fs.readFileSync(new URL(`../src/${name}.ts`,import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020}}).outputText,context);
 return context.exports;
}
const numbers=load('numbers');
const {draftLineAmount,draftSubtotal}=load('draft_amounts',name=>{assert.equal(name,'./numbers');return numbers});
const line=(quantityText,priceText,discountText='0')=>({quantityText,priceText,discountText});
test('entered line amounts use half-up rounding before line discounts',()=>{
 assert.equal(draftLineAmount(line('1.125','10.00','0.25')),1100);
 assert.equal(draftLineAmount(line('0.500','0.01')),1);
 assert.equal(draftSubtotal([line('0.500','0.01'),line('0.500','0.01')]),2);
 assert.equal(draftLineAmount(line('2','123.45','6.90')),24000);
});
test('rejects unusable amounts and reports subtotal limits without floating point overflow',()=>{
 for(const invalid of [line('0','10'),line('-1','10'),line('1','1','1.01'),line('1.0001','1'),line('1000000.001','0'),line('1','1000000000.01'),line('1000000','1000000000')])assert.throws(()=>draftLineAmount(invalid));
 assert.equal(draftLineAmount(line('10','1000000000')),1000000000000);
 assert.equal(draftLineAmount(line('10.001','1000000000','1000000')),1000000000000);
 assert.throws(()=>draftSubtotal([line('10','1000000000'),line('1','0.01')]));
});
