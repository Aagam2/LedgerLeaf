// Browser interaction coverage with an in-memory API. No server or database is opened.
import {build,transform} from 'esbuild';
import {chromium, expect} from '@playwright/test';
import fs from 'node:fs/promises';
import path from 'node:path';
import {tmpdir} from 'node:os';
import {fileURLToPath} from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const helper=await transform(await fs.readFile(path.join(root,'../tests/e2e/navigation.ts'),'utf8'),{loader:'ts',format:'esm'});
const {navigate}=await import('data:text/javascript;base64,'+Buffer.from(helper.code).toString('base64'));
const bundle = await build({
  stdin: {contents: `
    import React from 'react';
    import {createRoot} from 'react-dom/client';
    import {OperationsPage} from './src/operations';
    import {BanksPage} from './src/banks';
    import {RoleContext} from './src/permissions';
    import {Navigation} from './src/navigation';
    import {BillingPage} from './src/billing';
    import {Dashboard} from './src/dashboard';
    import {DocumentDownloads} from './src/document_downloads';
    import {useLanguage,setLanguage} from './src/i18n';
    window.setUiLanguage=setLanguage;
    function Workspace({role}){
      useLanguage();
      const [page,setPage]=React.useState('Overview');
      return <div className="shell"><aside><div className="brand">Ledgerleaf</div><Navigation role={role} hasBusiness page={page} onNavigate={setPage}/></aside><main><h1>{page}</h1>{page==='Billing'?<BillingPage businessId={1}/>:page==='Commercial documents'?<BillingPage businessId={1} commercial/>:<Dashboard businessId={1} onNavigate={setPage}/>}</main></div>;
    }
    const root=createRoot(document.getElementById('root'));
    window.renderPage=(page,role='ADMIN')=>root.render(
      <RoleContext.Provider value={role}>{page==='Downloads'?<DocumentDownloads pdfURL="/api/document-pdf?business_id=1&id=1"/>:page==='Workspace'?<Workspace role={role}/>:page==='Billing'?<BillingPage businessId={1}/>:page==='Orders'?<BillingPage businessId={1} commercial/>:page==='Banks'?
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
          if(resource==='documents'||resource==='commercial-documents'){
            const saved={...body,id:body.id||1,status:'DRAFT',number:'',business_name:'Test business',party_name:'Test party',business_address:'Local address',party_address:'Party address',tax_total:0,tax_enabled:false,
              lines:body.lines.map(l=>({...l,name:'Notebook',unit:'PCS',base_unit:'PCS',unit_factor:1000,amount:Math.round(l.quantity*l.price/1000)-l.discount,cgst:0,sgst:0,igst:0,cess:0})),
              total:window.reviewTotal||body.lines.reduce((n,l)=>n+Math.round(l.quantity*l.price/1000)-l.discount,0)};
            window.savedDocuments[resource+':'+saved.id]=saved;return saved;
          }
          if(resource==='finalize'||resource==='commercial-issue'){
            const source=resource==='finalize'?'documents':'commercial-documents';
            const saved=window.savedDocuments[source+':'+body.id];
            const posted={...saved,status:resource==='finalize'?'FINALIZED':'ISSUED',number:resource==='finalize'?'INV/001':'PO/001'};
            window.savedDocuments[source+':'+body.id]=posted;
            return window.minimalIssueResponse?{ok:true}:posted;
          }
          return {};
        }
        if(resource==='documents'||resource==='commercial-documents'){
          const id=new URLSearchParams(url.split('?')[1]).get('id');
          return id?window.savedDocuments[resource+':'+id]:(window.documentRows||[]);
        }
        if(['document-pdf','commercial-pdf','credit-note-pdf','dispatches'].includes(resource)&&url.includes('format=image-data')){
          window.imageRequests.push(url);
          if(window.imageError){const error=window.imageError;window.imageError='';throw new Error(error)}
          return window.printDocument;
        }
        if(resource==='banks')return [{code:'6001',name:'Trading bank',account_number:'1234'}];
        if(resource==='period-lock')return {closed_through:window.savedLock};
        if(resource==='year-close')return window.closedYears;
        if(resource==='parties')return [{id:3,name:'Supplier One',kind:'SUPPLIER',credit_days:0}];
        if(resource==='products')return [{id:7,name:'Notebook',sku:'N7',kind:'PRODUCT',active:1,sale_price:1000,purchase_price:600,unit:'PCS'}];
        if(resource==='warehouses')return [{id:1,name:'Main',active:1},{id:2,name:'Shop',active:1}];
        return [];
      }
    `, loader:'js'}));
  }}],
});
const browser = await chromium.launch({executablePath:process.env.CHROME_PATH||'/usr/bin/google-chrome',headless:true,ignoreDefaultArgs:['--headless=old'],args:['--headless=new','--no-sandbox','--disable-background-networking']});
try {
  async function pageFor(name, role='ADMIN', viewport={width:1280,height:900},setup={}) {
    const context=await browser.newContext({offline:true,viewport});
    const page=await context.newPage();
    await page.route('**/*',route=>route.abort());
    await page.setContent('<html><body><main id="root"></main></body></html>');
    // CSS is the prebuilt local Tailwind output, loaded without a request.
    const assets=path.resolve(root,'../internal/web/dist/assets');
    const css=(await fs.readdir(assets)).find(name=>name.endsWith('.css'));
    await page.addStyleTag({content:await fs.readFile(path.join(assets,css),'utf8')});
    await page.evaluate(setup=>{window.requests=[];window.savedLock='2026-03-31';window.closedYears=[];window.failNext='';window.savedDocuments={};window.reviewTotal=0;window.minimalIssueResponse=false;window.imageRequests=[];window.imageError='';Object.assign(window,setup)},setup);
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

  page=await pageFor('Workspace');
  if(process.env.LEDGERLEAF_UI_PREVIEW_DIR){
    await fs.mkdir(process.env.LEDGERLEAF_UI_PREVIEW_DIR,{recursive:true});
    await page.screenshot({path:path.join(process.env.LEDGERLEAF_UI_PREVIEW_DIR,'workspace.png'),fullPage:true});
  }
  let menu=page.getByRole('navigation',{name:'Main menu',exact:true});
  await expect(menu.getByRole('button',{name:'Billing',exact:true})).toBeVisible();
  await expect(menu.locator('button')).toHaveCount(29);
  await expect(menu.getByRole('button',{name:'Banks & periods',exact:true})).toHaveCount(0);
  await navigate(page,'Banks & periods');
  await expect(menu.getByRole('button',{name:'Banks & periods',exact:true})).toHaveAttribute('aria-current','page');
  await page.getByRole('button',{name:'Open Billing',exact:true}).click();
  await page.getByRole('button',{name:'+ Purchase bill',exact:true}).click();
  dialog=page.getByRole('dialog',{name:'New purchase bill',exact:true});
  await expect(dialog.getByLabel('Document type',{exact:true})).toHaveValue('PURCHASE');
  await expect(dialog.getByLabel('Supplier',{exact:true})).toBeVisible();
  if(process.env.LEDGERLEAF_UI_PREVIEW_DIR)await page.screenshot({path:path.join(process.env.LEDGERLEAF_UI_PREVIEW_DIR,'purchase.png'),fullPage:true});
  await dialog.getByLabel('Supplier',{exact:true}).selectOption('3');
  await dialog.getByLabel('Add line item',{exact:true}).selectOption('7');
  await dialog.getByRole('button',{name:'Save draft',exact:true}).click();
  await expect(dialog).toHaveCount(0);
  expect(await page.evaluate(()=>window.requests.at(-1).body.kind)).toBe('PURCHASE');
  await page.evaluate(()=>window.setUiLanguage('hi'));
  await expect(page.getByRole('navigation',{name:'मुख्य मेनू',exact:true})).toBeVisible();
  await expect(page.getByRole('button',{name:'+ खरीद बिल',exact:true})).toBeVisible();
  await page.evaluate(()=>window.setUiLanguage('gu'));
  await expect(page.getByRole('navigation',{name:'મુખ્ય મેનુ',exact:true})).toBeVisible();
  await expect(page.getByRole('button',{name:'+ ખરીદી બિલ',exact:true})).toBeVisible();
  await page.context().close();
  console.log('PASS grouped navigation keeps all 29 destinations and opens an explicit purchase draft');

  page=await pageFor('Workspace','BILLER',{width:390,height:844});
  menu=page.getByRole('navigation',{name:'Main menu',exact:true});
  await expect(menu.getByRole('button',{name:'Billing',exact:true})).toBeVisible();
  await expect(menu.locator('button').filter({hasText:'Backup & restore'})).toHaveCount(0);
  await expect(menu.locator('summary').filter({hasText:/^Accounting$/})).toHaveCount(0);
  await menu.getByRole('button',{name:'Billing',exact:true}).click();
  await expect(page.getByRole('button',{name:'+ Purchase bill',exact:true})).toHaveCount(0);
  await page.getByRole('button',{name:'+ New bill',exact:true}).click();
  await expect(page.getByRole('dialog').getByLabel('Document type',{exact:true})).toHaveValue('SALE');
  const mobileBounds=await page.getByRole('dialog').boundingBox();
  expect(mobileBounds.x+mobileBounds.width).toBeLessThanOrEqual(390);
  await page.context().close();
  console.log('PASS mobile biller menu and bill actions retain role restrictions');


  page=await pageFor('Billing');
  await page.getByRole('button',{name:'+ New bill',exact:true}).click();
  dialog=page.getByRole('dialog',{name:'New sales bill',exact:true});
  await dialog.getByLabel('Customer',{exact:true}).selectOption('3');
  await dialog.getByLabel('Add line item',{exact:true}).selectOption('7');
  const quantity=dialog.getByLabel('Notebook Quantity',{exact:true});
  await expect(quantity).toBeFocused();
  await quantity.fill('1.125');
  await dialog.getByLabel('Notebook Discount',{exact:true}).fill('0.25');
  await expect(dialog.getByLabel('Notebook Item amount',{exact:true})).toHaveText('₹ 11.00');
  await expect(dialog.locator('.bill-entry-actions').getByText('Item subtotal: ₹ 11.00',{exact:true})).toBeVisible();
  await dialog.getByLabel('Add line item',{exact:true}).selectOption('7');
  await expect(quantity).toBeFocused();
  await expect(dialog.getByRole('status')).toContainText('already on the bill');
  await expect(dialog.locator('tbody tr')).toHaveCount(1);
  if(process.env.LEDGERLEAF_UI_PREVIEW_DIR)await page.screenshot({path:path.join(process.env.LEDGERLEAF_UI_PREVIEW_DIR,'invoice-entry.png'),fullPage:true});
  await page.evaluate(()=>window.failNext='Draft rejected');
  await dialog.getByRole('button',{name:'Save & review',exact:true}).click();
  await expect(dialog.getByRole('alert')).toContainText('Draft rejected');
  await expect(quantity).toHaveValue('1.125');
  // A distinct server total proves review doesn't present the entered subtotal as final.
  await page.evaluate(()=>window.reviewTotal=12345);
  await dialog.getByRole('button',{name:'Save & review',exact:true}).click();
  dialog=page.getByRole('dialog',{name:'Review document',exact:true});
  await expect(dialog.getByRole('heading',{name:'Total ₹ 123.45',exact:true})).toBeVisible();
  await expect(dialog.getByRole('link',{name:'Download PDF',exact:true})).toHaveCount(0);
  expect(await page.evaluate(()=>window.requests.filter(r=>r.resource==='finalize').length)).toBe(0);
  await dialog.getByRole('button',{name:'Edit draft',exact:true}).click();
  dialog=page.getByRole('dialog',{name:'New sales bill',exact:true});
  await expect(dialog.getByLabel('Notebook Quantity',{exact:true})).toHaveValue('1.125');
  await dialog.getByRole('button',{name:'Save & review',exact:true}).click();
  dialog=page.getByRole('dialog',{name:'Review document',exact:true});
  expect(await page.evaluate(()=>window.requests.at(-1).body.id)).toBe(1);
  if(process.env.LEDGERLEAF_UI_PREVIEW_DIR)await page.screenshot({path:path.join(process.env.LEDGERLEAF_UI_PREVIEW_DIR,'invoice-review.png'),fullPage:true});
  await page.evaluate(()=>window.failNext='Not enough stock');
  await dialog.getByRole('button',{name:'Finalize',exact:true}).click();
  await expect(dialog.getByRole('alert')).toContainText('Not enough stock');
  await expect(dialog.getByRole('button',{name:'Edit draft',exact:true})).toBeVisible();
  // Reject the confirmation once; the backend must receive no further finalize request.
  page.removeAllListeners('dialog');page.once('dialog',event=>event.dismiss());
  await dialog.getByRole('button',{name:'Finalize',exact:true}).click();
  expect(await page.evaluate(()=>window.requests.filter(r=>r.resource==='finalize').length)).toBe(1);
  page.on('dialog',event=>event.accept());
  await dialog.getByRole('button',{name:'Finalize',exact:true}).click();
  dialog=page.getByRole('dialog',{name:'INV/001',exact:true});
  await expect(dialog.getByRole('link',{name:'Download PDF',exact:true})).toHaveAttribute('href','/api/document-pdf?business_id=1&id=1');
  await expect(dialog.getByRole('button',{name:'Edit draft',exact:true})).toHaveCount(0);
  await page.context().close();
  console.log('PASS invoice entry focuses quantities, shows exact entered amounts, retains failed drafts and reviews server totals before confirmed finalization/printing');

  page=await pageFor('Orders','ADMIN',{width:390,height:844});
  await page.getByRole('button',{name:'+ Purchase order',exact:true}).click();
  dialog=page.getByRole('dialog',{name:'Purchase order',exact:true});
  await expect(dialog.getByLabel('Commercial type',{exact:true})).toHaveValue('PURCHASE_ORDER');
  await expect(dialog.getByLabel('Document type',{exact:true})).toHaveValue('PURCHASE');
  await dialog.getByLabel('Supplier',{exact:true}).selectOption('3');
  await dialog.getByLabel('Add line item',{exact:true}).selectOption('7');
  await expect(dialog.getByLabel('Notebook Unit price',{exact:true})).toHaveValue('6.00');
  await dialog.getByRole('button',{name:'Save & review',exact:true}).click();
  dialog=page.getByRole('dialog',{name:'Review document',exact:true});
  await expect(dialog.getByRole('heading',{name:'Total ₹ 6.00',exact:true})).toBeVisible();
  await page.evaluate(()=>window.minimalIssueResponse=true);
  await dialog.getByRole('button',{name:'Issue',exact:true}).click();
  dialog=page.getByRole('dialog',{name:'PO/001',exact:true});
  await expect(dialog.getByRole('link',{name:'Download PDF',exact:true})).toHaveAttribute('href','/api/commercial-pdf?business_id=1&id=1');
  const orderBounds=await dialog.boundingBox();expect(orderBounds.x+orderBounds.width).toBeLessThanOrEqual(390);
  await page.context().close();
  console.log('PASS purchase order uses supplier/purchase prices and handles issue/retry response before print on mobile');


  const readonlyDraft={id:1,status:'DRAFT',kind:'SALE',number:'',date:'2026-10-22',due_date:'2026-10-22',total:1000,party_id:3,warehouse_id:1,notes:'',lines:[{product_id:7,name:'Notebook',unit:'PCS',quantity:1000,price:1000,discount:0,amount:1000}]};
  page=await pageFor('Billing','VIEWER',{width:1280,height:900},{documentRows:[{...readonlyDraft,party:'Test party'}],savedDocuments:{'documents:1':readonlyDraft}});
  await expect(page.getByRole('button',{name:'+ New bill',exact:true})).toHaveCount(0);
  await page.getByRole('button',{name:'View',exact:true}).click();
  dialog=page.getByRole('dialog');
  await expect(dialog.getByRole('button',{name:'Edit draft',exact:true})).toHaveCount(0);
  await expect(dialog.getByRole('button',{name:'Finalize',exact:true})).toHaveCount(0);
  await expect(dialog.getByRole('link',{name:'Download PDF',exact:true})).toHaveCount(0);
  expect(await page.evaluate(()=>window.requests.length)).toBe(0);
  await page.context().close();
  console.log('PASS viewer draft inspection exposes no editing, finalization or draft-print actions');


  const logo=(await fs.readFile(path.join(root,'../tests/fixtures/logo.png'))).toString('base64');
  const printDocument={Title:'TAX INVOICE - SALE - RETURNED / CANCELLED',Number:'INV/001',Business:'Café Traders',Address:'Saved business address\nGSTIN: FROZEN-GSTIN',Party:'Customer One',PartyAddress:'Saved customer address',Date:'2026-10-22',DueDate:'2026-10-30',TaxSummary:'CGST 9.00 / SGST 9.00 / Cess 1.00',Adjustments:'Rounding -0.01',Total:'118.99',Notes:'Frozen terms, footer and bank details',Logo:logo,LogoFormat:'png',Lines:[{Name:'Saved item',Quantity:'1.000 PCS',Price:'100.00',Discount:'0.00',Amount:'100.00',Tax:'HSN 1234 / GST 18%'}]};
  page=await pageFor('Downloads','ADMIN',{width:1280,height:900},{printDocument});
  await page.evaluate(()=>{
    window.drawnTexts=[];window.logoDraws=0;
    const text=CanvasRenderingContext2D.prototype.fillText,draw=CanvasRenderingContext2D.prototype.drawImage;
    CanvasRenderingContext2D.prototype.fillText=function(value,...args){window.drawnTexts.push(value);return text.call(this,value,...args)};
    CanvasRenderingContext2D.prototype.drawImage=function(...args){window.logoDraws++;return draw.apply(this,args)};
  });
  await expect(page.locator('.document-downloads .actions > :first-child')).toHaveText('Download PDF');
  await expect(page.getByRole('button',{name:'Export image (PNG)',exact:true})).not.toHaveClass(/primary/);
  await page.evaluate(()=>window.imageError='Export temporarily unavailable');
  await page.getByRole('button',{name:'Export image (PNG)',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('Export temporarily unavailable');
  await expect(page.getByRole('link',{name:'Download PDF',exact:true})).toBeVisible();
  const pngPromise=page.waitForEvent('download');
  await page.getByRole('button',{name:'Export image (PNG)',exact:true}).click();
  const pngDownload=await pngPromise;
  expect(pngDownload.suggestedFilename()).toBe('Ledgerleaf-INV-001.png');
  const exportDir=process.env.LEDGERLEAF_UI_PREVIEW_DIR||await fs.mkdtemp(path.join(tmpdir(),'ledgerleaf-png-test-'));
  await fs.mkdir(exportDir,{recursive:true});
  const pngPath=path.join(exportDir,'invoice-export.png');await pngDownload.saveAs(pngPath);
  const png=await fs.readFile(pngPath);
  expect(png.subarray(0,8).toString('hex')).toBe('89504e470d0a1a0a');
  expect(png.readUInt32BE(16)).toBe(1200);expect(png.readUInt32BE(20)).toBe(1697);
  const painted=await page.evaluate(()=>({text:window.drawnTexts.join('\n'),logos:window.logoDraws}));
  for(const value of ['RETURNED / CANCELLED','FROZEN-GSTIN','HSN 1234','Cess 1.00','Total: INR 118.99','Frozen terms, footer and bank details','Rounding -0.01'])expect(painted.text).toContain(value);
  expect(painted.logos).toBe(1);
  expect(await page.evaluate(()=>window.imageRequests.at(-1))).toBe('document-pdf?business_id=1&id=1&format=image-data');
  await page.evaluate(document=>{window.printDocument={...document,Lines:Array.from({length:100},(_,i)=>({Name:'Item '+String(i+1).padStart(3,'0'),Quantity:'1 PCS',Price:'1.00',Discount:'0.00',Amount:'1.00',Tax:'CGST and SGST details'})),Notes:'FINAL DOCUMENT NOTES'};window.drawnTexts=[]},printDocument);
  await page.getByRole('button',{name:'Export image (PNG)',exact:true}).click();
  await expect(page.getByRole('status')).toContainText('Images ready.');
  const imageLinks=page.locator('.image-downloads a');const pageCount=await imageLinks.count();expect(pageCount).toBeGreaterThan(1);
  const longText=await page.evaluate(()=>window.drawnTexts.join('\n'));
  for(let i=1;i<=100;i++)expect(longText).toContain('Item '+String(i).padStart(3,'0'));
  expect(longText).toContain('FINAL DOCUMENT NOTES');
  const lastPromise=page.waitForEvent('download');await imageLinks.last().click();
  const lastDownload=await lastPromise;expect(lastDownload.suggestedFilename()).toBe(`Ledgerleaf-INV-001-page-${pageCount}.png`);
  await lastDownload.saveAs(path.join(exportDir,'invoice-last-page.png'));
  if(!process.env.LEDGERLEAF_UI_PREVIEW_DIR)await fs.rm(exportDir,{recursive:true,force:true});
  await page.context().close();
  console.log('PASS PDF stays primary; real PNG downloads preserve frozen logo/tax/notes, errors can retry, and long documents include every item across separate page images');

} finally {await browser.close()}
