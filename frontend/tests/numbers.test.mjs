import test from 'node:test';
import assert from 'node:assert/strict';
import ts from 'typescript';
import vm from 'node:vm';
import fs from 'node:fs';
const context={exports:{}};
vm.runInNewContext(ts.transpileModule(fs.readFileSync(new URL('../src/numbers.ts',import.meta.url),'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020}}).outputText,context);
const {scaled,displayScaled}=context.exports;
test('decimal input is exact in minor units',()=>{assert.equal(scaled('0.29',2),29);assert.equal(scaled('1000.01',2),100001);assert.equal(scaled('1.125',3),1125);assert.equal(scaled('0002.1',3),2100);assert.equal(displayScaled(29,2),'0.29');assert.equal(displayScaled(-5,3),'-0.005')});
test('rejects malformed, excessive precision and overflowing input',()=>{for(const value of ['','-1','NaN','1e6','0.001','1.2.3','99999999999999999999999'])assert.throws(()=>scaled(value,2))});

test('large report aggregates format without precision loss',()=>assert.equal(displayScaled('9007199254740993',2),'90071992547409.93'));
test('integer fields use zero decimal places',()=>{assert.equal(scaled('30',0),30);assert.equal(displayScaled(30,0),'30');assert.throws(()=>scaled('1.5',0))});
