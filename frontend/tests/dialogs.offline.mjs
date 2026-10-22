// Browser interaction coverage with an in-memory API. No server or database is opened.
import {build} from 'esbuild';
import {chromium, expect} from '@playwright/test';
import fs from 'node:fs/promises';
import path from 'node:path';
import {fileURLToPath} from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const bundle = await build({
  stdin: {contents: `
    import React from 'react';
    import {createRoot} from 'react-dom/client';
    import {OperationsPage} from './src/operations';
    import {BanksPage} from './src/banks';
    import {RoleContext} from './src/permissions';
    const root=createRoot(document.getElementById('root'));
    window.renderPage=(page,role='ADMIN')=>root.render(
      <RoleContext.Provider value={role}>{page==='Banks'?
        <BanksPage businessId={1}/>:
        <OperationsPage businessId={1} page={page}/>
      }</RoleContext.Provider>);
  `, resolveDir: root, loader: 'tsx'},
  bundle: true, write: false, format: 'iife',
  define: {'process.env.NODE_ENV': '"production"'},
  plugins: [{name: 'in-memory-api', setup(b) {
    b.onResolve({filter: /^\.\/api$/}, () => ({path:'api',namespace:'mock'}));
    b.onLoad({filter: /.*/, namespace:'mock'}, () => ({contents: `
      export async function api(url,body){
        const resource=url.split('?')[0];
        if(body){
          window.requests.push({resource,body});
          if(window.failNext){const error=window.failNext;window.failNext='';throw new Error(error)}
          if(resource==='period-lock')window.savedLock=body.closed_through;
          if(resource==='year-close'&&body.preview)return {date:body.date,net_profit:10000,lines:[{Account:'4000',Debit:10000,Credit:0},{Account:'3100',Debit:0,Credit:10000}]};
          if(resource==='year-close'){window.closedYears.push({date:body.date,net_profit:10000,reference:'year-close:'+body.date});window.savedLock=body.date}
          return {};
        }
        if(resource==='banks')return [{code:'6001',name:'Trading bank',account_number:'1234'}];
        if(resource==='period-lock')return {closed_through:window.savedLock};
        if(resource==='year-close')return window.closedYears;
        if(resource==='products')return [{id:7,name:'Notebook',sku:'N7',kind:'PRODUCT',active:1}];
        if(resource==='warehouses')return [{id:1,name:'Main',active:1},{id:2,name:'Shop',active:1}];
        return [];
      }
    `, loader:'js'}));
  }}],
});
const browser = await chromium.launch({executablePath:process.env.CHROME_PATH||'/usr/bin/google-chrome',headless:true,ignoreDefaultArgs:['--headless=old'],args:['--headless=new','--no-sandbox','--disable-background-networking']});
try {
  async function pageFor(name, role='ADMIN', viewport={width:1280,height:900}) {
    const context=await browser.newContext({offline:true,viewport});
    const page=await context.newPage();
    await page.route('**/*',route=>route.abort());
    await page.setContent('<html><body><main id="root"></main></body></html>');
    // CSS is the prebuilt local Tailwind output, loaded without a request.
    const assets=path.resolve(root,'../internal/web/dist/assets');
    const css=(await fs.readdir(assets)).find(name=>name.endsWith('.css'));
    await page.addStyleTag({content:await fs.readFile(path.join(assets,css),'utf8')});
    await page.evaluate(()=>{window.requests=[];window.savedLock='2026-03-31';window.closedYears=[];window.failNext=''});
    await page.addScriptTag({content:bundle.outputFiles[0].text});
    await page.evaluate(({name,role})=>window.renderPage(name,role),{name,role});
    page.on('dialog',dialog=>dialog.accept());
    return page;
  }
  let page=await pageFor('Expenses');
  await expect(page.getByLabel('Amount (₹)',{exact:true})).toHaveCount(0);
  await page.getByRole('button',{name:'+ Record expense',exact:true}).click();
  let dialog=page.getByRole('dialog',{name:'Record expense',exact:true});
  await dialog.getByLabel('Amount (₹)',{exact:true}).fill('125.25');
  await dialog.getByLabel('Description',{exact:true}).fill('Office supplies');
  await dialog.getByLabel('Payment account',{exact:true}).selectOption('6001');
  await page.evaluate(()=>window.failNext='Posting rejected');
  await dialog.getByRole('button',{name:'Post expense',exact:true}).click();
  await expect(dialog.getByRole('alert')).toContainText('Posting rejected');
  await expect(dialog.getByLabel('Description',{exact:true})).toHaveValue('Office supplies');
  const firstKey=await page.evaluate(()=>window.requests.at(-1).body.key);
  await dialog.getByRole('button',{name:'Post expense',exact:true}).click();
  await expect(dialog).toHaveCount(0);
  const expense=await page.evaluate(()=>window.requests.at(-1).body);
  expect(expense.amount).toBe(12525);
  expect(expense.account).toBe('6001');
  expect(expense.key).toBe(firstKey);
  await page.getByRole('button',{name:'+ Record expense',exact:true}).click();
  await expect(page.getByRole('dialog').getByLabel('Description',{exact:true})).toHaveValue('');
  await page.context().close();
  console.log('PASS expense dialog retains failed input and request key; success closes and resets');

  page=await pageFor('Stock changes','ADMIN',{width:390,height:844});
  await page.getByRole('button',{name:'+ Post stock movement',exact:true}).click();
  dialog=page.getByRole('dialog',{name:'Post stock movement',exact:true});
  await dialog.getByLabel('Stock product',{exact:true}).selectOption('7');
  await dialog.getByLabel('Source warehouse',{exact:true}).selectOption('1');
  await dialog.getByLabel('Destination warehouse',{exact:true}).selectOption('2');
  await dialog.getByLabel('Quantity',{exact:true}).fill('-1');
  await dialog.getByLabel('Reason',{exact:true}).fill('Move to shop');
  await dialog.getByRole('button',{name:'Post stock movement',exact:true}).click();
  await expect(dialog.getByRole('alert')).toBeVisible();
  expect(await page.evaluate(()=>window.requests.length)).toBe(0);
  await dialog.getByLabel('Quantity',{exact:true}).fill('1.125');
  // Check the mobile dialog itself fits the viewport; its body may scroll internally.
  const bounds=await dialog.boundingBox();
  expect(bounds.x).toBeGreaterThanOrEqual(0);
  expect(bounds.x+bounds.width).toBeLessThanOrEqual(390);
  await dialog.getByRole('button',{name:'Post stock movement',exact:true}).click();
  await expect(dialog).toHaveCount(0);
  const stock=await page.evaluate(()=>window.requests.at(-1).body);
  expect(stock.kind).toBe('TRANSFER');expect(stock.quantity).toBe(1125);
  expect(stock.warehouse_id).toBe(1);expect(stock.target_id).toBe(2);
  await page.context().close();
  console.log('PASS stock dialog rejects negative quantities and posts exact transfer quantities on mobile');

  page=await pageFor('Banks');
  await expect(page.getByText('Financial postings are locked through 2026-03-31.',{exact:true})).toBeVisible();
  await page.getByRole('button',{name:'Change period lock',exact:true}).click();
  dialog=page.getByRole('dialog',{name:'Change period lock',exact:true});
  await dialog.getByLabel('Closed through',{exact:true}).fill('2026-09-30');
  await dialog.getByRole('button',{name:'Cancel',exact:true}).click();
  await expect(page.getByText('Financial postings are locked through 2026-03-31.',{exact:true})).toBeVisible();
  expect(await page.evaluate(()=>window.requests.length)).toBe(0);
  await page.getByRole('button',{name:'Change period lock',exact:true}).click();
  dialog=page.getByRole('dialog',{name:'Change period lock',exact:true});
  await expect(dialog.getByLabel('Closed through',{exact:true})).toHaveValue('2026-03-31');
  await dialog.getByLabel('Closed through',{exact:true}).fill('2026-09-30');
  await page.evaluate(()=>window.failNext='Lock rejected');
  await dialog.getByRole('button',{name:'Save period lock',exact:true}).click();
  await expect(dialog.getByRole('alert')).toContainText('Lock rejected');
  await expect(dialog.getByLabel('Closed through',{exact:true})).toHaveValue('2026-09-30');
  await dialog.getByRole('button',{name:'Save period lock',exact:true}).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByText('Financial postings are locked through 2026-09-30.',{exact:true})).toBeVisible();

  await page.getByRole('button',{name:'Review year closing',exact:true}).click();
  dialog=page.getByRole('dialog',{name:'Financial-year closing',exact:true});
  await dialog.getByLabel('Financial year-end date',{exact:true}).fill('2027-03-31');
  await expect(dialog.getByRole('button',{name:'Close financial year',exact:true})).toBeDisabled();
  await dialog.getByRole('button',{name:'Preview year closing',exact:true}).click();
  await expect(dialog.getByRole('status')).toContainText('100.00');
  await dialog.getByLabel('Financial year-end date',{exact:true}).fill('2028-03-31');
  await expect(dialog.getByRole('button',{name:'Close financial year',exact:true})).toBeDisabled();
  await dialog.getByRole('button',{name:'Preview year closing',exact:true}).click();
  await dialog.getByRole('button',{name:'Close financial year',exact:true}).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByText('Financial postings are locked through 2028-03-31.',{exact:true})).toBeVisible();
  await expect(page.getByText('2028-03-31 · Net profit transferred ₹ 100.00',{exact:true})).toBeVisible();
  await page.context().close();
  console.log('PASS period lock discard/save and year-closing preview invalidation/confirmation');

  page=await pageFor('Banks','ACCOUNTANT');
  await expect(page.getByRole('heading',{name:'Financial period lock',exact:true})).toBeVisible();
  await expect(page.getByRole('button',{name:'Change period lock',exact:true})).toHaveCount(0);
  await expect(page.getByRole('button',{name:'Review year closing',exact:true})).toHaveCount(0);
  await page.context().close();
  console.log('PASS accountant sees saved period lock without administrator actions');
} finally {await browser.close()}
