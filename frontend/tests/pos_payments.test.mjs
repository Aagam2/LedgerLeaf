import test from 'node:test';
import assert from 'node:assert/strict';
import ts from 'typescript';
import vm from 'node:vm';
import fs from 'node:fs';
function load(name,require=()=>{throw new Error('Unexpected import')}){const context={exports:{},require};vm.runInNewContext(ts.transpileModule(fs.readFileSync(new URL(`../src/${name}.ts`,import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020}}).outputText,context);return context.exports;}
const numbers=load('numbers');
const {paymentSummary}=load('pos_payments',name=>{assert.equal(name,'./numbers');return numbers;});
test('cash change does not increase posted payment',()=>{const s=paymentSummary([{mode:'CASH',amount:'80.25'}],8025,'100');assert.equal(s.change,1975);assert.equal(s.lines[0].amount,8025);assert.equal(s.remaining,0);});
test('split tender measures only cash and preserves paise',()=>{const s=paymentSummary([{mode:'CASH',amount:'30.25'},{mode:'UPI',amount:'50'}],8025,'50');assert.equal(s.change,1975);assert.equal(s.cash,3025);assert.equal(s.allocated,8025);});
test('shortfalls and remaining balance are explicit',()=>{const s=paymentSummary([{mode:'CASH',amount:'30.25'}],8025,'20');assert.equal(s.change,-1025);assert.equal(s.remaining,5000);});
test('blank tender uses exact cash and malformed amounts fail',()=>{assert.equal(paymentSummary([{mode:'CASH',amount:'0.29'}],29,'').change,0);for(const value of ['-1','abc','0.001'])assert.throws(()=>paymentSummary([{mode:'CASH',amount:value}],29,''));});
