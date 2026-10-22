import {navigate} from './navigation';
import {test, expect, Page} from '../../frontend/node_modules/@playwright/test';
import {spawn, ChildProcess} from 'node:child_process';
import {mkdtemp, rm, writeFile} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {resolve, join} from 'node:path';

let child: ChildProcess;
let directory: string;
let launchURL: string;
let origin: string;

test.beforeEach(async ({page}) => {
  directory = await mkdtemp(join(tmpdir(), 'ledgerleaf-regression-'));
  child = spawn(process.env.LEDGERLEAF_TEST_BINARY || resolve('../release/ledgerleaf-linux'), ['--data-dir', directory, '--no-browser']);
  launchURL = await new Promise<string>((resolveURL, reject) => {
    let output = '';
    const timer = setTimeout(() => reject(new Error('Application startup timeout')), 10000);
    child.stdout!.on('data', chunk => {
      output += chunk;
      const found = output.match(/http:\/\/127\.0\.0\.1:\d+\/#launch=[a-f0-9]+/);
      if (found) { clearTimeout(timer); resolveURL(found[0]); }
    });
    child.once('error', reject);
    child.once('exit', code => { clearTimeout(timer); reject(new Error(`Application exited: ${code}`)); });
  });
  origin = new URL(launchURL).origin;
  await page.route('**/*', route => route.request().url().startsWith(origin + '/') ? route.continue() : route.abort());
  await page.goto(launchURL);
  await expect(page.getByRole('heading', {name: 'Your business starts here.'})).toBeVisible();
  await post(page, 'businesses', {name: 'Test Shop', state_code: '27'}, 201);
  await page.reload();
  await expect(page.getByRole('heading', {name: 'Welcome to Test Shop'})).toBeVisible();
});

test.afterEach(async () => {
  if (child && child.exitCode === null) {
    const ended = new Promise(resolveExit => child.once('exit', resolveExit));
    child.kill('SIGINT');
    await ended;
  }
  if (directory) await rm(directory, {recursive: true, force: true});
});

async function post(page: Page, path: string, body: unknown, status = 200) {
  const response = await page.request.post(origin + '/api/' + path, {
    headers: {'Origin': origin, 'X-Ledgerleaf': '1'}, data: body,
  });
  expect(response.status(), await response.text()).toBe(status);
  return response.json();
}

async function seedProduct(page: Page) {
  await post(page, 'products?business_id=1', {name: 'Notebook', sku: 'BOOK', kind: 'PRODUCT', unit: 'PCS', sale_price: 10000});
}

async function seedBill(page: Page, kind: 'SALE'|'PURCHASE', quantity = 1000) {
  await post(page, 'parties?business_id=1', {name: kind === 'SALE' ? 'Customer One' : 'Supplier One', kind: kind === 'SALE' ? 'CUSTOMER' : 'SUPPLIER'});
  await seedProduct(page);
  return post(page, 'documents?business_id=1', {kind, party_id: 1, warehouse_id: 1, date: '2026-10-15', due_date: '2026-10-20', key: 'browser-document-1234', lines: [{product_id: 1, quantity, price: 10000}]});
}

test('purchase receipts and partial payments reject overpayment without losing the bill', async ({page}) => {
  const bill = await seedBill(page, 'PURCHASE');
  await post(page, 'finalize?business_id=1', {id: bill.id});
  await navigate(page, 'Billing');
  await page.getByRole('button', {name: 'Payment', exact: true}).click();
  await page.getByLabel('Amount (₹)').fill('40');
  await page.getByLabel('Date', {exact: true}).fill('2026-10-15');
  await page.getByRole('button', {name: 'Record payment'}).click();
  await expect(page.getByRole('cell', {name: 'PARTIALLY PAID', exact: true})).toBeVisible();
  await expect(page.getByRole('cell', {name: '₹ 60.00', exact: true})).toBeVisible();
  await page.getByRole('button', {name: 'Payment', exact: true}).click();
  await page.getByLabel('Amount (₹)').fill('61');
  await page.getByLabel('Date', {exact: true}).fill('2026-10-15');
  await page.getByRole('button', {name: 'Record payment'}).click();
  await expect(page.getByRole('alert')).toContainText('exceeds outstanding');
  await page.getByLabel('Amount (₹)').fill('60');
  await page.getByRole('button', {name: 'Record payment'}).click();
  await expect(page.getByRole('cell', {name: 'PAID', exact: true})).toBeVisible();
  await navigate(page, 'Inventory');
  await expect(page.getByRole('cell', {name: '1.000', exact: true})).toBeVisible();
  await expect(page.getByRole('cell', {name: '₹ 100.00', exact: true})).toBeVisible();
});

test('stock shortage displays an error and preserves the editable draft', async ({page}) => {
  await seedBill(page, 'SALE', 2000);
  await navigate(page, 'Billing');
  page.once('dialog', dialog => dialog.accept());
  await page.getByRole('button', {name: 'Finalize', exact: true}).click();
  await expect(page.getByRole('alert')).toContainText('insufficient stock');
  await expect(page.getByRole('cell', {name: 'DRAFT', exact: true}).last()).toBeVisible();
  await page.getByRole('button', {name: 'Edit', exact: true}).click();
  await expect(page.getByLabel('Notebook Quantity')).toHaveValue('2.000');
  await page.getByLabel('Notebook Quantity').fill('1.000');
  await page.getByRole('button', {name: 'Save draft'}).click();
  await expect(page.getByRole('cell', {name: '₹ 100.00', exact: true})).toBeVisible();
});

test('duplicate SKU validation, editing and business isolation work in the UI', async ({page}) => {
  await seedProduct(page);
  const second = await post(page, 'businesses', {name: 'Other Shop', state_code: '27'}, 201);
  await page.reload();
  await navigate(page, 'Products');
  await page.getByRole('button', {name: '+ Add product'}).click();
  await page.getByLabel('Name', {exact: true}).fill('Duplicate');
  await page.getByLabel('SKU', {exact: true}).fill('BOOK');
  await page.getByRole('button', {name: 'Save record'}).click();
  await expect(page.getByRole('alert')).toContainText('UNIQUE constraint');
  await page.getByRole('button', {name: 'Cancel', exact: true}).click();
  await page.getByRole('button', {name: 'Edit', exact: true}).click();
  await page.getByLabel('Name', {exact: true}).fill('Notebook revised');
  await page.getByLabel('Active', {exact: true}).uncheck();
  await page.getByRole('button', {name: 'Save record'}).click();
  await expect(page.getByRole('cell', {name: 'Notebook revised'})).toBeVisible();
  await expect(page.getByRole('cell', {name: 'Archived'})).toBeVisible();
  await page.getByLabel('Current business').selectOption(String(second.id));
  await expect(page.getByText('No records found.', {exact: true})).toBeVisible();
  await expect(page.getByRole('cell', {name: 'Notebook revised'})).toHaveCount(0);
  await page.getByLabel('Current business').selectOption('1');
  await expect(page.getByRole('cell', {name: 'Notebook revised'})).toBeVisible();
});

test('session refresh, dark mode and mobile layout work without external resources', async ({page}) => {
  await page.getByRole('button', {name: 'Dark appearance', exact: false}).click();
  await expect(page.locator('html')).toHaveClass('dark');
  await page.reload();
  await expect(page.getByRole('heading', {name: 'Welcome to Test Shop'})).toBeVisible();
  await expect(page.locator('html')).toHaveClass('dark');
  await page.setViewportSize({width: 390, height: 844});
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await navigate(page, 'Products');
  await expect(page.getByRole('button', {name: '+ Add product'})).toBeVisible();
});

test('paid invoice full return restores inventory and closes the receivable',async({page})=>{
 const bill=await seedBill(page,'SALE');
 await post(page,'opening-stock?business_id=1',{product_id:1,warehouse_id:1,quantity:1000,value:6000,date:'2026-10-15',key:'browser-return-opening'});
 await post(page,'finalize?business_id=1',{id:bill.id});
 await post(page,'payments?business_id=1',{document_id:bill.id,amount:10000,date:'2026-10-15',mode:'CASH',key:'browser-return-payment'});
 await navigate(page, 'Billing');
 await page.getByRole('button',{name:'Return / cancel',exact:true}).click();
 await page.getByLabel('Return date').fill('2026-10-16');
 await page.getByLabel('Reason',{exact:true}).fill('All goods returned');
 page.once('dialog',d=>d.accept());
 await page.getByRole('button',{name:'Preview return',exact:true}).click();
 await expect(page.getByRole('status')).toContainText('Refund');
 await page.getByRole('button',{name:'Post return',exact:true}).click();
 await expect(page.getByRole('cell',{name:'RETURNED / CANCELLED'})).toBeVisible();
 await expect(page.getByRole('button',{name:'Payment',exact:true})).toHaveCount(0);
 await navigate(page, 'Inventory');
 await expect(page.getByRole('cell',{name:'1.000',exact:true})).toBeVisible();
 await expect(page.getByRole('cell',{name:'₹ 60.00',exact:true})).toBeVisible();
});

test('create, download, import, verify and restore a backup through the browser',async({page})=>{
 await navigate(page, 'Backup & restore');
 await page.getByRole('button',{name:'Create backup',exact:true}).click();
 await expect(page.getByRole('status')).toContainText('Created and verified');
 await page.getByRole('button',{name:'Verify',exact:true}).click();
 await expect(page.getByRole('status')).toContainText('Integrity verified');
 const downloadPromise=page.waitForEvent('download');
 await page.getByRole('link',{name:'Download',exact:true}).click();
 const download=await downloadPromise;
 const saved=join(directory,'download.db');
 await download.saveAs(saved);
 await page.getByLabel('Import a downloaded backup (.db, up to 512 MB)').setInputFiles(saved);
 await expect(page.getByRole('status')).toContainText('Imported and verified');
 await expect(page.getByRole('button',{name:'Restore all businesses'})).toHaveCount(2);
 await post(page,'businesses',{name:'Created after snapshot',state_code:'27'},201);
 page.once('dialog',d=>d.accept());
 await page.getByRole('button',{name:'Restore all businesses'}).first().click();
 await expect(page.getByRole('heading',{name:'Welcome to Test Shop'})).toBeVisible();
 const businesses=await page.request.get(origin+'/api/businesses');
 expect((await businesses.json()).map((b:any)=>b.name)).toEqual(['Test Shop']);
 await navigate(page, 'Backup & restore');
 await expect(page.getByRole('button',{name:'Restore all businesses'})).toHaveCount(3);
});

test('warehouse transfer, expense and financial report reconcile',async({page})=>{
 await seedProduct(page);
 await post(page,'warehouses?business_id=1',{name:'Branch'});
 await post(page,'opening-stock?business_id=1',{product_id:1,warehouse_id:1,quantity:10000,value:60000,date:'2026-10-15',key:'browser-transfer-opening'});
 await navigate(page, 'Stock changes');
 await page.getByLabel('Stock product').selectOption({label:'Notebook · BOOK'});
 await page.getByLabel('Source warehouse',{exact:true}).selectOption({label:'Main warehouse'});
 await page.getByLabel('Destination warehouse',{exact:true}).selectOption({label:'Branch'});
 await page.getByLabel('Quantity',{exact:true}).fill('2');
 await page.getByLabel('Date',{exact:true}).fill('2026-10-16');
 await page.getByLabel('Reason',{exact:true}).fill('Branch replenishment');
 await page.getByRole('button',{name:'Post stock movement'}).click();
 await expect(page.getByRole('status')).toContainText('Stock movement posted');
 await navigate(page, 'Expenses');
 await page.getByLabel('Amount (₹)').fill('50');
 await page.getByLabel('Date',{exact:true}).fill('2026-10-16');
 await page.getByLabel('Description',{exact:true}).fill('Delivery expense');
 await page.getByRole('button',{name:'Post expense'}).click();
 await expect(page.getByRole('cell',{name:'Delivery expense'})).toBeVisible();
 await navigate(page, 'Reports');
 await page.getByLabel('To',{exact:true}).fill('2026-10-16');
 await expect(page.getByRole('heading',{name:'Net profit: ₹ -50.00'})).toBeVisible();
 const download=await page.getByRole('link',{name:'Export CSV'}).getAttribute('href');
 const response=await page.request.get(origin+download);
 expect(response.headers()['content-type']).toContain('text/csv');
 expect(await response.text()).toContain('Expenses');
});

test('administrator enables passwords and viewer cannot change business data',async({page})=>{
 await navigate(page, 'Users & permissions');
 await page.getByRole('button',{name:'Add account',exact:true}).click();
 await page.getByLabel('Username',{exact:true}).fill('admin');
 await page.getByLabel('Password',{exact:true}).fill('Admin-Password-123!');
 await page.getByRole('button',{name:'Save account',exact:true}).click();
 await expect(page.getByRole('heading',{name:'Signed in as admin'})).toBeVisible();
 await page.getByRole('button',{name:'Add account',exact:true}).click();
 await page.getByLabel('Username',{exact:true}).fill('viewer');
 await page.getByLabel('Password',{exact:true}).fill('Viewer-Password-123!');
 await page.getByLabel('Role',{exact:true}).selectOption('VIEWER');
 await page.getByRole('button',{name:'Save account',exact:true}).click();
 await expect(page.getByRole('cell',{name:'viewer',exact:true})).toBeVisible();
 await page.getByRole('button',{name:'Sign out',exact:true}).click();
 await expect(page.getByRole('heading',{name:'Sign in to Ledgerleaf',exact:true})).toBeVisible();
 await expect(page.getByRole('heading',{name:'Sign in to Ledgerleaf'})).toBeVisible();
 await page.getByLabel('Username',{exact:true}).fill('viewer');
 await page.getByLabel('Password',{exact:true}).fill('Viewer-Password-123!');
 await page.getByRole('button',{name:'Sign in',exact:true}).click();
 await expect(page.getByRole('heading',{name:'Welcome to Test Shop'})).toBeVisible();
 await expect(page.getByRole('button',{name:'Edit business',exact:true})).toHaveCount(0);
 await expect(page.getByRole('navigation').getByRole('button', {name: 'Backup & restore',exact:false})).toHaveCount(0);
 await navigate(page, 'Products');
 await expect(page.getByRole('button',{name:'+ Add product'})).toHaveCount(0);
 const denied=await page.request.post(origin+'/api/parties?business_id=1',{headers:{Origin:origin,'X-Ledgerleaf':'1'},data:{name:'Not allowed',kind:'CUSTOMER'}});
 expect(denied.status()).toBe(403);
 await page.reload();
 await expect(page.getByRole('heading',{name:'Welcome to Test Shop'})).toBeVisible();
});

test('business profile editing and manual journal entry persist',async({page})=>{
 await page.getByRole('button',{name:'Edit business',exact:true}).click();
 await page.getByLabel('Business name',{exact:true}).fill('Updated Shop');
 await page.getByText('Contact, bank and invoice details',{exact:false}).click();
 await page.getByLabel('Bank name',{exact:true}).fill('Test Bank');
 await page.getByLabel('Invoice terms',{exact:true}).fill('Payment within seven days');
 await page.getByRole('button',{name:'Save business',exact:true}).click();
 await expect(page.getByRole('heading',{name:'Welcome to Updated Shop'})).toBeVisible();
 await page.reload();
 await expect(page.getByRole('heading',{name:'Welcome to Updated Shop'})).toBeVisible();
 await navigate(page, 'Journal entries');
 await page.getByRole('button',{name:'+ Post journal',exact:true}).click();
 await page.getByLabel('Date',{exact:true}).fill('2026-10-16');
 await page.getByLabel('Description',{exact:true}).fill('Opening cash capital');
 await page.getByLabel('Debit 1',{exact:true}).fill('1000');
 await page.getByLabel('Credit 2',{exact:true}).fill('1000');
 await page.getByRole('button',{name:'Post journal',exact:true}).click();
 await expect(page.getByRole('status')).toContainText('Saved to the ledger');
 await navigate(page, 'Reports');
 await page.getByLabel('Report',{exact:true}).selectOption('balance-sheet');
 await page.getByLabel('To',{exact:true}).fill('2026-10-16');
 await expect(page.getByRole('heading',{name:'Assets: ₹ 1000.00'})).toBeVisible();
});

test('GST configuration, tax invoice and partial credit note reconcile in browser',async({page})=>{
 await seedProduct(page);
 await post(page,'businesses',{id:1,name:'Test Shop',state_code:'27',gstin:'27AAPFU0939F1ZV',tax_scheme:'REGULAR'},200);
 await post(page,'parties?business_id=1',{name:'GST Customer',kind:'CUSTOMER',state_code:'27'});
 await post(page,'opening-stock?business_id=1',{product_id:1,warehouse_id:1,quantity:2000,value:12000,date:'2026-10-15',key:'browser-gst-opening'});
 await navigate(page, 'GST configuration');
 await page.getByRole('button',{name:'+ Configure tax',exact:true}).click();
 await page.getByLabel('Tax item').selectOption('1');
 await page.getByLabel('Effective date').fill('2026-01-01');
 await page.getByLabel('HSN / SAC').fill('1001');
 await page.getByRole('button',{name:'Save tax configuration'}).click();
 await expect(page.getByRole('cell',{name:'1001',exact:true})).toBeVisible();
 await navigate(page, 'Billing');
 await page.getByRole('button',{name:'+ New bill'}).click();
 await page.getByLabel('Customer',{exact:true}).selectOption('1');
 await page.getByLabel('Document date').fill('2026-10-15');
 await page.getByLabel('Due date').fill('2026-10-20');
 await page.getByText('Tax, pricing and delivery options',{exact:true}).click();
 await page.getByLabel('Place of supply (state code)').fill('27');
 await page.getByLabel('Add line item').selectOption('1');
 await page.getByLabel('Notebook Quantity').fill('2');
 await page.getByRole('button',{name:'Save draft'}).click();
 page.once('dialog',d=>d.accept());
 await page.getByRole('button',{name:'Finalize',exact:true}).click();
 await expect(page.getByRole('cell',{name:'₹ 236.00',exact:true})).toHaveCount(2);
 await page.getByRole('button',{name:'View',exact:true}).click();
 await expect(page.getByText('GST / cess: ₹ 36.00')).toBeVisible();
 await page.getByRole('button',{name:'Close',exact:true}).click();
 await page.getByRole('button',{name:'Return / cancel'}).click();
 await page.getByLabel('Notebook return quantity').fill('1');
 await page.getByLabel('Reason',{exact:true}).fill('One item returned');
 await page.getByLabel('Return date').fill('2026-10-15');
 await page.getByRole('button',{name:'Preview return',exact:true}).click();
 await expect(page.getByRole('status')).toContainText('Credit ₹ 118.00');
 page.once('dialog',d=>d.accept());
 await page.getByRole('button',{name:'Post return',exact:true}).click();
 await expect(page.getByRole('cell',{name:'₹ 118.00',exact:true})).toBeVisible();
 await navigate(page, 'Credit notes');
 await expect(page.getByRole('link',{name:'Download PDF'})).toBeVisible();
});

test('quotation issues without stock posting and converts once into a billing draft',async({page})=>{
 await seedProduct(page);await post(page,'parties?business_id=1',{name:'Quote Customer',kind:'CUSTOMER'});
 await navigate(page, 'Commercial documents');
 await page.getByRole('button',{name:'+ New document'}).click();
 await page.getByLabel('Customer',{exact:true}).selectOption('1');
 await page.getByLabel('Add line item').selectOption('1');
 await page.getByRole('button',{name:'Save draft'}).click();
 page.once('dialog',d=>d.accept());await page.getByRole('button',{name:'Issue',exact:true}).click();
 await expect(page.getByRole('cell',{name:'ISSUED',exact:true})).toBeVisible();
 await page.getByRole('button',{name:'View',exact:true}).click();
 const downloadPromise=page.waitForEvent('download');await page.getByRole('link',{name:'Download PDF'}).click();expect((await downloadPromise).suggestedFilename()).toContain('.pdf');
 await page.getByRole('button',{name:'Close',exact:true}).click();
 await page.getByRole('button',{name:'Convert to bill draft'}).click();
 await expect(page.getByText('Linked bill #1')).toBeVisible();
 await navigate(page, 'Billing');
 await expect(page.getByRole('button',{name:'Finalize',exact:true})).toBeVisible();
});

test('POS barcode, hold/resume and split payment produce a thermal receipt',async({page})=>{
 await seedProduct(page);await post(page,'opening-stock?business_id=1',{product_id:1,warehouse_id:1,quantity:2000,value:12000,date:'2026-10-15',key:'pos-opening-stock-key'});
 await navigate(page, 'POS');
 await page.getByRole('button',{name:'Use walk-in customer'}).click();
 await expect(page.getByLabel('POS customer')).toHaveValue('1');
 await page.getByLabel('POS date').fill('2026-10-15');
 await page.getByLabel('Scan barcode / search products').fill('BOOK');
 await expect(page.getByRole('button',{name:'Notebook · ₹ 100.00',exact:true})).toBeVisible();
 await page.getByLabel('Scan barcode / search products').press('Enter');
 await page.getByRole('button',{name:'Hold bill',exact:true}).click();
 await expect(page.getByRole('status')).toContainText('Held draft #1');
 await page.getByRole('button',{name:'Resume #1',exact:true}).click();
 await expect(page.getByLabel('Notebook POS quantity')).toHaveValue('1.000');
 await page.getByRole('button',{name:'Review checkout'}).click();
 await expect(page.getByRole('heading',{name:'Checkout total ₹ 100.00'})).toBeVisible();
 await page.getByLabel('Payment amount 1').fill('60');
 await page.getByRole('button',{name:'Add payment part'}).click();
 await page.getByLabel('Payment amount 2').fill('40');
 await page.getByLabel('Cash tendered ₹',{exact:true}).fill('80');
 await expect(page.getByRole('status').filter({hasText:'Change due'})).toHaveText('Change due ₹ 20.00');
 await page.keyboard.press('F9');
 await expect(page.getByRole('status').filter({hasText:'payment recorded'})).toContainText('payment recorded');
 const downloadPromise=page.waitForEvent('download');await page.getByRole('link',{name:'Print 80 mm receipt'}).click();expect((await downloadPromise).suggestedFilename()).toContain('.pdf');
 const payments=await page.request.get(origin+'/api/payments?business_id=1');const paymentRows=await payments.json();expect(paymentRows.length).toBe(2);expect(paymentRows.reduce((n:number,p:any)=>n+p.amount,0)).toBe(10000);
});

test('bulk import validates CSV and downloads spreadsheet and PDF reports',async({page})=>{
 await navigate(page, 'Import & export');
 await page.getByLabel('Import file',{exact:true}).setInputFiles({name:'products.csv',mimeType:'text/csv',buffer:Buffer.from('name,kind,sku,unit,sale_price\nImported item,PRODUCT,IM1,PCS,10000\n')});
 await page.getByRole('button',{name:'Import new records'}).click();
 await expect(page.getByRole('status')).toContainText('1 records imported successfully');
 const download=page.waitForEvent('download');await page.getByRole('link',{name:'Export active products as XLSX'}).click();expect((await download).suggestedFilename()).toBe('Ledgerleaf-products.xlsx');
 await navigate(page, 'Products');
 await expect(page.getByRole('cell',{name:'Imported item',exact:true})).toBeVisible();
 await navigate(page, 'Reports');
 await page.getByLabel('Report',{exact:true}).selectOption('ageing');
 const pdf=page.waitForEvent('download');await page.getByRole('link',{name:'Export PDF'}).click();expect((await pdf).suggestedFilename()).toBe('Ledgerleaf-ageing.pdf');
});

test('automatic backup policy persists and duplicate bill stays an unposted draft',async({page})=>{
 await navigate(page, 'Backup & restore');
 await page.getByLabel('Enable automatic backups').check();
 await page.getByLabel('Backup interval (hours)').fill('12');
 await page.getByLabel('Automatic backups to retain').fill('3');
 await page.getByRole('button',{name:'Save backup schedule'}).click();
 await expect(page.getByRole('status')).toContainText('Backup schedule saved');
 await page.reload();
 await navigate(page, 'Backup & restore');
 await expect(page.getByLabel('Enable automatic backups')).toBeChecked();
 await expect(page.getByLabel('Backup interval (hours)')).toHaveValue('12');
 const original=await seedBill(page,'PURCHASE');
 await post(page,'finalize?business_id=1',{id:original.id});
 await navigate(page, 'Billing');
 await page.getByRole('button',{name:'Duplicate',exact:true}).click();
 await expect(page.getByLabel('Notebook Quantity')).toHaveValue('1.000');
 await page.getByRole('button',{name:'Save draft',exact:true}).click();
 await expect(page.getByRole('cell',{name:'DRAFT',exact:true})).toBeVisible();
 const stock=await page.request.get(origin+'/api/stock?business_id=1');
 expect((await stock.json())[0].quantity).toBe(1000);
});

test('biller navigation opens POS and hides administrator and accounting actions',async({page})=>{
 await post(page,'auth/users',{username:'admin',password:'Admin-Password-123!',role:'ADMIN',active:1});
 await post(page,'auth/users',{username:'biller',password:'Biller-Password-123!',role:'BILLER',active:1});
 await post(page,'auth/logout',{});
 await page.reload();
 await page.getByLabel('Username',{exact:true}).fill('biller');
 await page.getByLabel('Password',{exact:true}).fill('Biller-Password-123!');
 await page.getByRole('button',{name:'Sign in',exact:true}).click();
 await expect(page.getByRole('heading',{name:'Point of sale'})).toBeVisible();
 await expect(page.getByRole('navigation').getByRole('button', {name: 'Backup & restore',exact:false})).toHaveCount(0);
 await expect(page.getByRole('navigation').getByRole('button', {name: 'Reports',exact:false})).toHaveCount(0);
 await navigate(page, 'Products');
 await expect(page.getByRole('button',{name:'+ Add product'})).toHaveCount(0);
 await navigate(page, 'Billing');
 await page.getByRole('button',{name:'+ New bill'}).click();
 await expect(page.getByLabel('Document type',{exact:true})).toBeDisabled();
 await page.getByRole('button',{name:'Close',exact:true}).click();
 await navigate(page, 'Users & permissions');
 await page.getByRole('button',{name:'Change your password',exact:true}).click();
 await expect(page.getByRole('dialog',{name:'Change your password',exact:true}).getByRole('button',{name:'Change password',exact:true})).toBeVisible();
});

test('partial goods receipt becomes supplier invoice without receiving stock twice',async({page})=>{
 await post(page,'parties?business_id=1',{name:'Receipt Supplier',kind:'SUPPLIER'});
 await seedProduct(page);
 const order=await post(page,'commercial-documents?business_id=1',{document_type:'PURCHASE_ORDER',kind:'PURCHASE',party_id:1,warehouse_id:1,date:'2026-10-15',due_date:'2026-10-15',key:'receipt-browser-order',lines:[{product_id:1,quantity:3000,price:6000}]});
 await post(page,'commercial-issue?business_id=1',{id:order.id});
 await navigate(page, 'Goods receipts');
 await page.getByRole('button',{name:'New goods receipt',exact:true}).click();
 await page.getByLabel('Purchase order ID').fill(String(order.id));
 await page.getByRole('button',{name:'Load purchase order'}).click();
 await page.getByLabel('Notebook receipt quantity').fill('1');
 await page.getByRole('button',{name:'Post goods receipt'}).click();
 await expect(page.getByRole('status')).toContainText('Stock received');
 await page.getByRole('button',{name:'Create supplier invoice draft'}).click();
 await expect(page.getByRole('status')).toContainText('Billing draft');
 await navigate(page, 'Billing');
 page.once('dialog',d=>d.accept());
 await page.getByRole('button',{name:'Finalize',exact:true}).click();
 await expect(page.getByRole('cell',{name:'UNPAID',exact:true})).toBeVisible();
 await navigate(page, 'Inventory');
 await expect(page.getByRole('cell',{name:'1.000',exact:true})).toBeVisible();
 await navigate(page, 'Goods receipts');
 await expect(page.getByText('Posted bill #1')).toBeVisible();
 await page.getByRole('button',{name:'New goods receipt',exact:true}).click();
 await page.getByLabel('Purchase order ID').fill(String(order.id));
 await page.getByRole('button',{name:'Load purchase order'}).click();
 await expect(page.getByLabel('Notebook receipt quantity')).toHaveValue('2.000');
});

test('administrator assigns a user to one business through the UI',async({page})=>{
 await post(page,'businesses',{name:'Private Shop',state_code:'27'},201);
 await post(page,'auth/users',{username:'admin',password:'Admin-Password-123!',role:'ADMIN',active:1});
 await page.reload();
 await navigate(page, 'Users & permissions');
 await page.getByRole('button',{name:'Add account',exact:true}).click();
 await page.getByLabel('Username',{exact:true}).fill('limited');
 await page.getByLabel('Password',{exact:true}).fill('Limited-Password-123!');
 await page.getByLabel('Role',{exact:true}).selectOption('VIEWER');
 await page.getByLabel('Limit access to selected businesses').check();
 await page.getByRole('checkbox',{name:'Test Shop · #1',exact:true}).check();
 await page.getByRole('button',{name:'Save account',exact:true}).click();
 await expect(page.getByRole('cell',{name:'limited',exact:true})).toBeVisible();
 await page.getByRole('button',{name:'Sign out',exact:true}).click();
 await expect(page.getByRole('heading',{name:'Sign in to Ledgerleaf',exact:true})).toBeVisible();
 await page.getByLabel('Username',{exact:true}).fill('limited');
 await page.getByLabel('Password',{exact:true}).fill('Limited-Password-123!');
 await page.getByRole('button',{name:'Sign in',exact:true}).click();
 await expect(page.getByRole('heading',{name:'Welcome to Test Shop'})).toBeVisible();
 await expect(page.getByLabel('Current business').locator('option')).toHaveCount(2);
 const response=await page.request.get(origin+'/api/stock?business_id=2');
 expect(response.status()).toBe(403);
});

test('alternate units persist on bills and POS while stock uses base units',async({page})=>{
 const purchase=await seedBill(page,'PURCHASE',24000);await post(page,'finalize?business_id=1',{id:purchase.id});
 await post(page,'parties?business_id=1',{name:'Box Customer',kind:'CUSTOMER'});
 await navigate(page, 'Unit conversions');
 await page.getByRole('button',{name:'+ Add conversion',exact:true}).click();
 await page.getByLabel('Conversion product',{exact:true}).selectOption('1');
 await page.getByLabel('Alternate unit',{exact:true}).fill('BOX');
 await page.getByLabel('Base units per alternate unit').fill('12');
 await page.getByRole('button',{name:'Add conversion',exact:true}).click();
 await expect(page.getByRole('cell',{name:'12.000 PCS',exact:true})).toBeVisible();
 await navigate(page, 'Billing');
 await page.getByRole('button',{name:'+ New bill'}).click();
 await page.getByLabel('Customer',{exact:true}).selectOption('2');
 await page.getByLabel('Add line item').selectOption('1');
 await page.getByLabel('Unit for item 1').selectOption('BOX');
 await expect(page.getByLabel('Notebook Unit price')).toHaveValue('1200.00');
 await page.getByRole('button',{name:'Save draft'}).click();
 await navigate(page, 'POS');
 await page.getByRole('button',{name:'Resume #2'}).click();
 await expect(page.getByLabel('Unit for item 1')).toHaveValue('BOX');
 await page.getByRole('button',{name:'Review checkout'}).click();
 await expect(page.getByRole('heading',{name:'Checkout total ₹ 1200.00'})).toBeVisible();
 await page.getByRole('button',{name:'Finalize & record payment'}).click();
 await expect(page.getByRole('status').filter({hasText:'Sale finalized'})).toContainText('Sale finalized');
 await navigate(page, 'Inventory');
 await expect(page.getByRole('cell',{name:'12.000',exact:true})).toBeVisible();
});

test('logo and product image uploads persist and invoice logo survives removal',async({page})=>{
 await page.getByText('View business details',{exact:true}).click();
 await page.getByLabel('Business logo (PNG/JPEG, up to 512 KB)').setInputFiles(resolve('../tests/fixtures/logo.png'));
 await expect(page.getByRole('img',{name:'Business logo',exact:true})).toBeVisible();
 const bill=await seedBill(page,'SALE');
 await post(page,'opening-stock?business_id=1',{product_id:1,warehouse_id:1,quantity:1000,value:6000,date:'2026-10-15',key:'image-browser-opening'});
 await post(page,'finalize?business_id=1',{id:bill.id});
 await navigate(page, 'Products');
 await page.getByRole('button',{name:'Edit',exact:true}).click();
 await page.getByLabel('Product image (PNG/JPEG, up to 512 KB)').setInputFiles(resolve('../tests/fixtures/logo.png'));
 await expect(page.getByRole('img',{name:'Product image',exact:true})).toBeVisible();
 await page.getByRole('button',{name:'Save record'}).click();
 await expect(page.locator('table img')).toBeVisible();
 await page.reload();
 await page.getByText('View business details',{exact:true}).click();
 await expect(page.getByRole('img',{name:'Business logo',exact:true})).toBeVisible();
 await page.getByRole('button',{name:'Remove logo',exact:true}).click();
 await expect(page.getByRole('img',{name:'Business logo',exact:true})).toHaveCount(0);
 const pdf=await page.request.get(origin+`/api/document-pdf?business_id=1&id=${bill.id}`);
 expect(pdf.status()).toBe(200);
 const bytes=await pdf.body();expect(bytes.toString('latin1')).toContain('/Subtype /Image');
 await writeFile(resolve('../release/logo-invoice-preview.pdf'),bytes);
});

test('product and HSN summaries display normalized quantities and export',async({page})=>{
 await seedProduct(page);
 await post(page,'businesses',{id:1,name:'Test Shop',state_code:'27',gstin:'27AAPFU0939F1ZV',tax_scheme:'REGULAR'});
 await post(page,'parties?business_id=1',{name:'Summary Customer',kind:'CUSTOMER',state_code:'27'});
 await post(page,'taxes?business_id=1',{product_id:1,effective_date:'2026-01-01',hsn:'1001',treatment:'TAXABLE',rate:1800,cess:0},201);
 await post(page,'units?business_id=1',{product_id:1,name:'BOX',factor:12000});
 await post(page,'opening-stock?business_id=1',{product_id:1,warehouse_id:1,quantity:12000,value:60000,date:'2026-10-15',key:'summary-browser-stock'});
 const bill=await post(page,'documents?business_id=1',{kind:'SALE',party_id:1,warehouse_id:1,date:'2026-10-15',due_date:'2026-10-15',place_of_supply:'27',key:'summary-browser-bill',lines:[{product_id:1,quantity:1000,price:120000,sale_unit:'BOX'}]});
 await post(page,'finalize?business_id=1',{id:bill.id});
 await navigate(page, 'Reports');
 await page.getByLabel('Report',{exact:true}).selectOption('product-sales');
 await expect(page.getByRole('cell',{name:'12.000',exact:true})).toBeVisible();
 await expect(page.getByRole('cell',{name:'₹ 1200.00',exact:true})).toBeVisible();
 await page.getByLabel('Report',{exact:true}).selectOption('hsn-summary');
 await expect(page.getByRole('cell',{name:'1001',exact:true})).toBeVisible();
 await expect(page.getByRole('cell',{name:'18.00%',exact:true})).toBeVisible();
 const url=await page.getByRole('link',{name:'Export XLSX'}).getAttribute('href');
 const response=await page.request.get(origin+url);expect(response.status()).toBe(200);expect((await response.body()).subarray(0,2).toString()).toBe('PK');
});

test('invoice adjustments survive editing and POS checkout with exact returns',async({page})=>{
 const purchase=await seedBill(page,'PURCHASE',3000);await post(page,'finalize?business_id=1',{id:purchase.id});
 await post(page,'parties?business_id=1',{name:'Adjustment Customer',kind:'CUSTOMER'});
 await navigate(page, 'Billing');
 await page.getByRole('button',{name:'+ New bill'}).click();
 await page.getByLabel('Customer',{exact:true}).selectOption('2');
 await page.getByLabel('Add line item').selectOption('1');
 await page.getByLabel('Notebook Quantity').fill('3');
 await page.getByLabel('Notebook Discount').fill('1.00');
 await page.getByText('Discounts, charges and notes',{exact:true}).click();
 await page.getByLabel('Invoice discount ₹',{exact:true}).fill('10.00');
 await page.getByLabel('Allocated charges ₹',{exact:true}).fill('2.49');
 await page.getByLabel('Round total to nearest rupee').check();
 await page.getByRole('button',{name:'Save draft'}).click();
 await page.getByRole('button',{name:'Edit',exact:true}).click();
 await expect(page.getByLabel('Invoice discount ₹',{exact:true})).toHaveValue('10.00');
 await expect(page.getByLabel('Allocated charges ₹',{exact:true})).toHaveValue('2.49');
 await expect(page.getByLabel('Round total to nearest rupee')).toBeChecked();
 await page.getByRole('button',{name:'Close',exact:true}).click();
 await navigate(page, 'POS');
 await page.getByRole('button',{name:'Resume #2'}).click();
 await page.getByRole('button',{name:'Review checkout'}).click();
 await expect(page.getByRole('heading',{name:'Checkout total ₹ 291.00'})).toBeVisible();
 await page.getByRole('button',{name:'Finalize & record payment'}).click();
 await expect(page.getByRole('status').filter({hasText:'Sale finalized'})).toContainText('Sale finalized');
 const response=await page.request.get(origin+'/api/documents?business_id=1&id=2');
 const bill=await response.json();
 expect(bill.round_off).toBe(-49);expect(bill.invoice_discount).toBe(1000);expect(bill.lines[0].discount).toBe(100);
 const pdf=await page.request.get(origin+'/api/document-pdf?business_id=1&id=2');expect(pdf.status()).toBe(200);
 await writeFile(resolve('../release/adjusted-invoice-preview.pdf'),await pdf.body());
 let total=0,refund=0;
 for(let i=0;i<3;i++){
 const returned=await post(page,'reversals?business_id=1',{id:2,date:bill.date,reason:'Partial return',mode:'CASH',key:`adjustment-browser-return-${i}`,lines:[{product_id:1,quantity:1000}]});
 total+=returned.total;refund+=returned.refund;
 }
 expect(total).toBe(29100);expect(refund).toBe(29100);
});

test('administrator runs and downloads integrity diagnostics',async({page})=>{
 const purchase=await seedBill(page,'PURCHASE',3000);await post(page,'finalize?business_id=1',{id:purchase.id});
 await navigate(page, 'Backup & restore');
 await page.getByRole('button',{name:'Run integrity checks'}).click();
 await expect(page.getByRole('status')).toContainText('All integrity checks passed.');
 await expect(page.getByRole('cell',{name:'Inventory ledger reconciliation',exact:true})).toBeVisible();
 const downloaded=page.waitForEvent('download');
 await page.getByRole('button',{name:'Download diagnostic report'}).click();
 expect((await downloaded).suggestedFilename()).toBe('Ledgerleaf-diagnostics.json');
 await page.getByText('Recovery steps when checks fail',{exact:true}).click();
 await expect(page.getByText('Preserve a copy of the complete data directory',{exact:false})).toBeVisible();
});

test('direct receipts split and combine across invoices and corrections preserve stock',async({page})=>{
 await seedProduct(page);
 await post(page,'parties?business_id=1',{name:'Direct Supplier',kind:'SUPPLIER'});
 await navigate(page, 'Goods receipts');
 await page.getByRole('button',{name:'New goods receipt',exact:true}).click();
 await page.getByLabel('Receipt source',{exact:true}).selectOption('DIRECT');
 await page.getByLabel('Receipt supplier',{exact:true}).selectOption('1');
 await page.getByLabel('Add receipt item',{exact:true}).selectOption('1');
 await page.getByLabel('Notebook receipt quantity',{exact:true}).fill('3.000');
 await page.getByLabel('Notebook estimated unit price ₹',{exact:true}).fill('100.01');
 await page.getByRole('button',{name:'Post goods receipt',exact:true}).click();
 await expect(page.getByRole('status')).toContainText('Goods receipt #1 posted');
 const first=await (await page.request.get(origin+'/api/receipts?business_id=1&id=1')).json();
 await post(page,'receipts?business_id=1',{party_id:1,warehouse_id:1,date:first.date,key:'second-direct-browser-receipt',lines:[{product_id:1,quantity:2000,price:10000}]});
 await navigate(page, 'Billing');
 await navigate(page, 'Goods receipts');
 await page.getByLabel('Select receipt 1',{exact:true}).check();
 await page.getByLabel('Select receipt 2',{exact:true}).check();
 await page.getByRole('button',{name:'Match selected receipts',exact:true}).click();
 await page.getByLabel('Receipt 1 item 1 matched quantity').fill('1.000');
 await page.getByLabel('Receipt 2 item 1 matched quantity').fill('1.000');
 await page.getByRole('button',{name:'Create matched invoice draft',exact:true}).click();
 await expect(page.getByRole('status')).toContainText('Billing draft #1 created');
 await navigate(page, 'Billing');
 await page.getByRole('button',{name:'Edit',exact:true}).click();
 await page.getByLabel('Notebook Unit price').fill('101.00');
 await page.getByRole('button',{name:'Save draft',exact:true}).click();
 page.once('dialog',d=>d.accept());
 await page.getByRole('button',{name:'Finalize',exact:true}).click();
 await expect(page.getByRole('cell',{name:'UNPAID',exact:true})).toBeVisible();
 await navigate(page, 'Goods receipts');
 await expect(page.getByText('Partially invoiced',{exact:false})).toHaveCount(2);
 await page.getByLabel('Select receipt 1',{exact:true}).check();
 await page.getByLabel('Select receipt 2',{exact:true}).check();
 await page.getByRole('button',{name:'Match selected receipts',exact:true}).click();
 await expect(page.getByLabel('Receipt 1 item 1 matched quantity')).toHaveValue('2.000');
 await expect(page.getByLabel('Receipt 2 item 1 matched quantity')).toHaveValue('1.000');
 await page.getByRole('button',{name:'Create matched invoice draft',exact:true}).click();
 await expect(page.getByRole('status')).toContainText('Billing draft #2 created');
 await post(page,'finalize?business_id=1',{id:2});
 await post(page,'receipts?business_id=1',{party_id:1,warehouse_id:1,date:first.date,key:'correct-direct-browser-receipt',lines:[{product_id:1,quantity:1000,price:4000}]});
 await navigate(page, 'Billing');
 await navigate(page, 'Goods receipts');
 await page.getByRole('button',{name:'Correct receipt #3',exact:true}).click();
 await page.getByLabel('Correction reason',{exact:true}).fill('Duplicate delivery entry');
 page.once('dialog',d=>d.accept());
 await page.getByRole('button',{name:'Reverse goods receipt',exact:true}).click();
 await expect(page.getByRole('status')).toContainText('Receipt #3 reversed');
 await expect(page.getByRole('cell',{name:'REVERSED',exact:true})).toBeVisible();
 await navigate(page, 'Inventory');
 await expect(page.getByRole('cell',{name:'5.000',exact:true})).toBeVisible();
});

test('partial dispatch selection invoices and returns exact remaining stock',async({page})=>{
 await seedProduct(page);
 await post(page,'parties?business_id=1',{name:'Dispatch Customer',kind:'CUSTOMER'});
 await post(page,'opening-stock?business_id=1',{product_id:1,warehouse_id:1,quantity:3000,value:100,date:'2026-10-15',key:'browser-dispatch-stock'});
 await navigate(page, 'Dispatch challans');
 await page.getByRole('button',{name:'New dispatch challan',exact:true}).click();
 await page.getByLabel('Dispatch customer',{exact:true}).selectOption({label:'Dispatch Customer'});
 await page.getByLabel('Dispatch date',{exact:true}).fill('2026-10-15');
 await page.getByLabel('Add dispatch item',{exact:true}).selectOption({label:'Notebook'});
 await page.getByLabel('Notebook dispatch quantity',{exact:true}).fill('3');
 page.once('dialog',d=>d.accept());await page.getByRole('button',{name:'Post dispatch challan',exact:true}).click();
 await expect(page.getByRole('status')).toContainText('Dispatch #1 posted.');
 await page.getByRole('button',{name:'Invoice dispatch #1',exact:true}).click();
 await page.getByLabel('Notebook selected dispatch quantity').fill('-1');
 await page.getByRole('button',{name:'Create partial invoice draft'}).click();
 await expect(page.getByRole('dialog').getByRole('alert')).toContainText('nonnegative number');
 expect(await (await page.request.get(origin+'/api/documents?business_id=1')).json()).toHaveLength(0);
 await page.getByLabel('Notebook selected dispatch quantity').fill('1');
 await page.setViewportSize({width:390,height:844});
 expect(await page.getByRole('dialog').evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBeTruthy();
 await page.screenshot({path:resolve('../release/verification-dispatch-dialogs/selection-mobile.png')});
 await page.setViewportSize({width:1280,height:720});
 await page.getByLabel('Invoice date',{exact:true}).fill('2026-10-15');
 await page.getByRole('button',{name:'Create partial invoice draft'}).click();
 await expect(page.getByRole('status')).toContainText('Billing draft #1 created');
 await post(page,'finalize?business_id=1',{id:1});
 await page.getByRole('button',{name:'Return dispatch #1',exact:true}).click();
 await expect(page.getByLabel('Notebook selected dispatch quantity')).toHaveValue('2.000');
 await page.getByLabel('Return date',{exact:true}).fill('2026-10-15');
 page.once('dialog',d=>d.accept());
 await page.getByRole('button',{name:'Return selected quantities'}).click();
 await expect(page.getByRole('status')).toContainText('Selected dispatched quantities returned');
 await page.getByRole('button',{name:'History #1',exact:true}).click();
 await expect(page.getByRole('cell',{name:'Invoice #1',exact:true})).toBeVisible();
 await expect(page.getByRole('cell',{name:'Warehouse return',exact:true})).toBeVisible();
 const stock=await (await page.request.get(origin+'/api/stock?business_id=1')).json();
 expect(stock[0].quantity).toBe(2000);expect(stock[0].value).toBe(67);
 const diagnostics=await post(page,'diagnostics',{});expect(diagnostics.status).toBe('pass');
});

test('journal import preview and year-close preview leave records unchanged until confirmed',async({page})=>{
 await navigate(page, 'Import & export');
 await page.getByLabel('Financial journal file').setInputFiles({name:'journals.CSV',mimeType:'text/csv',buffer:Buffer.from('reference,date,description,account,debit,credit\npreview-rent,2026-03-31,Rent,5200,2500,0\npreview-rent,2026-03-31,Rent,1000,0,2500\n')});
 await page.getByRole('button',{name:'Validate financial import'}).click();
 await expect(page.getByRole('cell',{name:'preview-rent',exact:true})).toBeVisible();
 expect(await (await page.request.get(origin+'/api/journals?business_id=1')).json()).toHaveLength(0);
 page.once('dialog',d=>d.accept());await page.getByRole('button',{name:'Post financial import'}).click();
 await expect(page.getByText('1 journals posted.',{exact:true})).toBeVisible();
 await navigate(page, 'Banks & periods');
 await page.getByLabel('Financial year-end date').fill('2026-03-31');
 await page.getByRole('button',{name:'Preview year closing'}).click();
 await expect(page.getByRole('status')).toContainText('Loss to transfer: ₹ 25.00');
 expect(await (await page.request.get(origin+'/api/year-close?business_id=1')).json()).toHaveLength(0);
 page.once('dialog',d=>d.accept());await page.getByRole('button',{name:'Close financial year',exact:true}).click();
 await expect(page.getByText('2026-03-31 · Net profit transferred ₹ -25.00',{exact:true})).toBeVisible();
 expect(await (await page.request.get(origin+'/api/year-close?business_id=1')).json()).toHaveLength(1);
});

test('encrypted backup downloads and imports after rejecting an incorrect password',async({page})=>{
 await navigate(page, 'Backup & restore');
 await page.getByRole('button',{name:'Create backup',exact:true}).click();
 await expect(page.getByRole('status').filter({hasText:'Created and verified'})).toBeVisible();
 await page.getByLabel('Snapshot to encrypt',{exact:true}).selectOption({index:1});
 await page.getByLabel('Backup encryption password').fill('browser-backup-password');
 const pending=page.waitForEvent('download');await page.getByRole('button',{name:'Download encrypted backup'}).click();
 const downloaded=await pending;expect(downloaded.suggestedFilename()).toMatch(/\.llbenc$/);
 const encrypted=join(directory,'export.llbenc');await downloaded.saveAs(encrypted);
 await page.getByLabel('Encrypted backup file',{exact:true}).setInputFiles(encrypted);
 await page.getByLabel('Backup encryption password').fill('incorrect-backup-password');
 await page.getByRole('button',{name:'Decrypt and verify backup'}).click();
 await expect(page.getByRole('alert')).toContainText('incorrect password');
 await page.getByLabel('Backup encryption password').fill('browser-backup-password');
 await page.getByRole('button',{name:'Decrypt and verify backup'}).click();
 await expect(page.getByRole('status').filter({hasText:'Decrypted and verified'})).toBeVisible();
 const backups=await (await page.request.get(origin+'/api/backups')).json();expect(backups.length).toBe(2);
 expect(await (await page.request.get(origin+'/api/businesses')).json()).toHaveLength(1);
});

test('billing selectors page beyond fifty and preserve saved selections outside search results',async({page})=>{
 test.setTimeout(90000);
 for(let i=1;i<=51;i++){
  const n=String(i).padStart(3,'0');
  await post(page,'parties?business_id=1',{name:`Customer ${n}`,kind:'CUSTOMER'});
  await post(page,'products?business_id=1',{name:`Product ${n}`,sku:`SKU-${n}`,kind:'PRODUCT',unit:'PCS',sale_price:100});
 }
 for(let i=1;i<=200;i++)await post(page,'warehouses?business_id=1',{name:`Warehouse ${String(i).padStart(3,'0')}`});
 await navigate(page, 'Billing');
 await page.getByRole('button',{name:'+ New bill'}).click();
 await page.getByRole('button',{name:'Next customer',exact:true}).click();
 await page.getByLabel('Customer',{exact:true}).selectOption({label:'Customer 051'});
 await page.getByRole('button',{name:'Previous customer',exact:true}).click();
 await expect(page.getByLabel('Customer',{exact:true})).toHaveValue('51');
 await searchLookup(page, 'Find party', 'Customer 001');
 await expect(page.getByLabel('Customer',{exact:true}).locator('option:checked')).toHaveText('Customer 051');
 await page.getByRole('button',{name:'Next add line item',exact:true}).click();
 await page.getByLabel('Add line item',{exact:true}).selectOption({label:'Product 051 · SKU-051'});
 await searchLookup(page, 'Find warehouse', 'Warehouse 200');
 await page.getByLabel('Warehouse',{exact:true}).selectOption({label:'Warehouse 200'});
 await page.getByRole('button',{name:'Save draft',exact:true}).click();
 await page.getByRole('button',{name:'Edit',exact:true}).click();
 await expect(page.getByLabel('Customer',{exact:true}).locator('option:checked')).toHaveText('Customer 051');
 await expect(page.getByLabel('Warehouse',{exact:true}).locator('option:checked')).toHaveText('Warehouse 200');
 const bill=await (await page.request.get(origin+'/api/documents?business_id=1&id=1')).json();
 expect(bill.party_id).toBe(51);expect(bill.warehouse_id).toBe(201);expect(bill.lines[0].product_id).toBe(51);
});

test('stock and configuration pickers reach later products and retain price selection',async({page})=>{
 for(let i=1;i<=51;i++)await post(page,'products?business_id=1',{name:`Item ${String(i).padStart(3,'0')}`,sku:`ITEM-${i}`,kind:'PRODUCT',unit:'PCS',sale_price:100});
 await navigate(page, 'Stock changes');
 await page.getByLabel('Operation',{exact:true}).selectOption('ADD');
 await page.getByRole('button',{name:'Next stock product',exact:true}).click();
 await page.getByLabel('Stock product',{exact:true}).selectOption('51');
 await page.getByLabel('Source warehouse',{exact:true}).selectOption('1');
 await page.getByLabel('Quantity',{exact:true}).fill('3');
 await page.getByLabel('Total cost of added stock (₹)').fill('6');
 await page.getByLabel('Reason',{exact:true}).fill('Later catalog item');
 await page.getByRole('button',{name:'Post stock movement'}).click();
 await expect(page.getByRole('status')).toContainText('Stock movement posted');
 await expect(page.getByLabel('Stock product',{exact:true})).toHaveValue('');
 await navigate(page, 'Unit conversions');
 await page.getByRole('button',{name:'+ Add conversion',exact:true}).click();
 await page.getByRole('button',{name:'Next conversion product',exact:true}).click();
 await page.getByLabel('Conversion product',{exact:true}).selectOption('51');
 await page.getByLabel('Alternate unit',{exact:true}).fill('BOX');
 await page.getByLabel('Base units per alternate unit').fill('3');
 await page.getByRole('button',{name:'Add conversion',exact:true}).click();
 await expect(page.getByRole('cell',{name:'3.000 PCS',exact:true})).toBeVisible();
 await navigate(page, 'Price lists');
 await page.getByRole('button',{name:'+ Add list price',exact:true}).click();
 await page.getByRole('button',{name:'Next product',exact:true}).click();
 await page.getByLabel('Product',{exact:true}).selectOption('51');
 await searchLookup(page, 'Find price-list product', 'Item 001');
 await expect(page.getByLabel('Product',{exact:true}).locator('option:checked')).toHaveText('Item 051 · ITEM-51');
 await page.getByLabel('List name',{exact:true}).fill('Wholesale');
 await page.getByLabel('Price (₹)',{exact:true}).fill('2');
 await page.getByRole('button',{name:'Save list price'}).click();
 await expect(page.getByRole('cell',{name:'Wholesale',exact:true})).toBeVisible();
 await expect(page.getByRole('cell',{name:'Item 051',exact:true})).toBeVisible();
});

test('invoice file previews atomically and posts purchase and sale together',async({page})=>{
 await seedProduct(page);
 await post(page,'parties?business_id=1',{name:'Import Supplier',kind:'SUPPLIER'});
 await post(page,'parties?business_id=1',{name:'Import Buyer',kind:'CUSTOMER'});
 await page.getByRole('button',{name:'Import',exact:false}).click();
 await page.getByLabel('Invoice import mode').selectOption('finalize');
 await page.getByLabel('Invoice import file').setInputFiles({name:'invoices.csv',mimeType:'text/csv',buffer:Buffer.from('reference,kind,date,due_date,party_id,warehouse_id,sku,quantity,price\nIMP-P,PURCHASE,2026-10-15,2026-10-15,1,1,BOOK,2000,6000\nIMP-S,SALE,2026-10-15,2026-10-15,2,1,BOOK,1000,10000\n')});
 await page.getByRole('button',{name:'Preview invoice import',exact:true}).click();
 await expect(page.getByText('2 invoices validated.',{exact:false})).toBeVisible();
 expect(await (await page.request.get(origin+'/api/documents?business_id=1')).json()).toHaveLength(0);
 page.once('dialog',dialog=>dialog.accept());
 await page.getByRole('button',{name:'Confirm invoice import',exact:true}).click();
 await expect(page.getByText('2 invoices finalized.',{exact:false})).toBeVisible();
 const bills=await (await page.request.get(origin+'/api/documents?business_id=1')).json();expect(bills).toHaveLength(2);expect(bills.every((b:any)=>b.status==='FINALIZED')).toBe(true);
});

test('two dispatches consolidate into one editable invoice without another stock issue',async({page})=>{
 await seedProduct(page);await post(page,'parties?business_id=1',{name:'Dispatch Buyer',kind:'CUSTOMER'});
 await post(page,'opening-stock?business_id=1',{product_id:1,warehouse_id:1,quantity:4000,value:24000,date:'2026-10-15',key:'consolidation-stock'});
 for(const key of ['consolidation-first','consolidation-second'])await post(page,'dispatches?business_id=1',{party_id:1,warehouse_id:1,date:'2026-10-15',key,lines:[{product_id:1,quantity:2000}]});
 await navigate(page, 'Dispatch challans');
 await page.getByLabel('Select dispatch 1 for consolidation').check();await page.getByLabel('Select dispatch 2 for consolidation').check();
 await page.getByRole('button',{name:'Combine selected dispatches',exact:false}).click();
 await page.getByRole('button',{name:'Load selected dispatch balances'}).click();
 await page.getByLabel('Dispatch 1 product 1 invoice quantity').fill('1');
 await page.getByLabel('Consolidated invoice date').fill('2026-10-15');
 await page.screenshot({path:resolve('../release/verification-dispatch-dialogs/consolidation-desktop.png')});
 await page.getByRole('button',{name:'Create consolidated invoice draft'}).click();
 await expect(page.getByRole('status')).toContainText('Consolidated billing draft #1 created');
 await navigate(page, 'Billing');await page.getByRole('button',{name:'Edit',exact:true}).click();
 await page.getByRole('button',{name:'Save draft',exact:true}).click();
 await post(page,'finalize?business_id=1',{id:1});
 const invoice=await (await page.request.get(origin+'/api/documents?business_id=1&id=1')).json();expect(invoice.dispatch_matches).toHaveLength(2);expect(invoice.lines[0].quantity).toBe(3000);expect(invoice.lines[0].cost).toBe(18000);
 const balance=await (await page.request.get(origin+'/api/dispatches?business_id=1&id=1&remaining=1')).json();expect(balance[0].remaining).toBe(1000);
});

test('configuration selectors reach later records and business search passes two hundred',async({page})=>{
 test.setTimeout(90000);
 for(let i=1;i<=51;i++){
  const n=String(i).padStart(3,'0');
  await post(page,'products?business_id=1',{name:`Paged ${n}`,sku:`PAGED-${n}`,kind:'PRODUCT',unit:'PCS',sale_price:100});
  await post(page,'parties?business_id=1',{name:`Party ${n}`,kind:'CUSTOMER'});
  await post(page,'variants?business_id=1',{name:`Group ${n}`});
 }
 await navigate(page, 'GST configuration');
 await page.getByRole('button',{name:'+ Configure tax',exact:true}).click();
 await page.getByRole('button',{name:'Next tax item',exact:true}).click();
 await page.getByLabel('Tax item',{exact:true}).selectOption('51');
 await page.getByLabel('Effective date',{exact:true}).fill('2026-10-18');
 await page.getByLabel('HSN / SAC',{exact:true}).fill('1001');
 await page.getByRole('button',{name:'Save tax configuration'}).click();
 await expect(page.getByRole('cell',{name:'Paged 051',exact:true})).toBeVisible();
 await navigate(page, 'Catalog settings');
 await page.getByRole('button',{name:'+ Assign variant',exact:true}).click();
 await page.getByRole('button',{name:'Next variant product',exact:true}).click();
 await page.getByLabel('Variant product',{exact:true}).selectOption('51');
 await page.getByRole('button',{name:'Next variant group',exact:true}).click();
 await page.getByLabel('Variant group',{exact:true}).selectOption('51');
 await page.getByLabel('Variant attributes').fill('color=blue');
 await page.getByRole('button',{name:'Assign variant',exact:true}).click();
 await expect(page.getByRole('cell',{name:'Group 051',exact:true})).toBeVisible();
 await navigate(page, 'Party balances');
 await page.getByRole('button',{name:'+ Party transaction',exact:true}).click();
 await page.getByRole('button',{name:'Next party',exact:true}).click();
 await page.getByLabel('Party',{exact:true}).selectOption('51');
 await page.getByLabel('Amount (₹)',{exact:true}).fill('100');
 await page.getByRole('button',{name:'Post party transaction'}).click();
 await expect(page.getByRole('cell',{name:'Party 051',exact:true})).toBeVisible();
 for(let i=2;i<=201;i++)await post(page,'businesses',{name:`Business ${String(i).padStart(3,'0')}`,state_code:'27'},201);
 await page.getByLabel('Find business',{exact:true}).fill('Business 201');
 await page.getByLabel('Current business').selectOption('201');
 await navigate(page, 'Overview');
 await expect(page.getByRole('heading',{name:'Welcome to Business 201',exact:true})).toBeVisible();
 await page.getByLabel('Find business',{exact:true}).fill('Test Shop');
 await expect(page.getByLabel('Current business')).toHaveValue('201');
 await page.getByLabel('Current business').selectOption('1');
 await expect(page.getByRole('heading',{name:'Welcome to Test Shop',exact:true})).toBeVisible();
});

test('daily workflow links, document filters and bill payment history', async ({page}) => {
  await page.getByRole('button', {name:'Open Billing',exact:true}).click();
  await expect(page.getByRole('heading', {name:'Billing',exact:true})).toBeVisible();
  const bill = await seedBill(page, 'PURCHASE');
  await post(page,'finalize?business_id=1',{id:bill.id});
  await post(page,'payments?business_id=1',{document_id:bill.id,date:'2026-10-15',amount:4000,mode:'CASH',key:'lookup-browser-pay01'});
  await page.reload();
  await page.getByRole('button',{name:'Open Billing',exact:true}).click();
  await page.getByLabel('Find documents').fill('does not exist');
  await expect(page.getByText('No documents match.',{exact:false})).toBeVisible();
  await page.getByLabel('Find documents').fill('Supplier One');
  await page.getByLabel('Filter document type').selectOption('PURCHASE');
  await page.getByLabel('Outstanding only').check();
  await expect(page.getByRole('cell',{name:'PARTIALLY PAID',exact:true})).toBeVisible();
  await page.getByRole('button',{name:'View',exact:true}).click();
  const history=page.getByRole('region',{name:'Payment history'});
  await expect(history.getByRole('cell',{name:'₹ 40.00',exact:true})).toBeVisible();
  await expect(history.getByRole('cell',{name:'1000',exact:true})).toBeVisible();
  await page.getByRole('button',{name:'Close',exact:true}).click();
  await post(page,'payments?business_id=1',{document_id:bill.id,date:'2026-10-15',amount:6000,mode:'CASH',key:'lookup-browser-pay02'});
  await page.getByLabel('Find documents').fill('Supplier');
  await expect(page.getByText('No documents match.',{exact:false})).toBeVisible();
  await page.getByLabel('Outstanding only').uncheck();
  await expect(page.getByRole('cell',{name:'PAID',exact:true})).toBeVisible();
});

test('dialogs keep business and product workflows compact and browse values', async ({page})=>{
 await page.getByRole('button',{name:'+ Add business',exact:true}).click();
 const business=page.getByRole('dialog',{name:'Add business',exact:true});
 await expect(business).toBeVisible();
 await business.getByLabel('Business name',{exact:true}).fill('Second shop');
 await business.getByLabel('State code',{exact:true}).fill('27');
 await business.getByRole('button',{name:'Create business →'}).click();
 await expect(business).toHaveCount(0);
 await expect(page.getByRole('heading',{name:'Welcome to Second shop'})).toBeVisible();
 await page.getByLabel('Current business').selectOption('1');
 await seedProduct(page);
 await navigate(page, 'Products');
 const add=page.getByRole('button',{name:'+ Add product'});
 await add.click();
 await page.getByRole('dialog').getByLabel('Name',{exact:true}).fill('Unsaved');
 page.once('dialog',d=>d.dismiss());await page.keyboard.press('Escape');
 await expect(page.getByRole('dialog')).toBeVisible();
 page.once('dialog',d=>d.accept());await page.keyboard.press('Escape');
 await expect(page.getByRole('dialog')).toHaveCount(0);await expect(add).toBeFocused();
 await navigate(page, 'Billing');
 await page.getByRole('button',{name:'+ New bill',exact:true}).click();
 await page.getByRole('button',{name:'Find a product or service',exact:true}).click();
 const lookup=page.getByRole('dialog',{name:'Choose add line item',exact:true});
 await lookup.getByLabel('Search add line item').fill('Notebook');
 await lookup.getByRole('button',{name:'Notebook · BOOK Select →',exact:true}).click();
 await expect(lookup).toHaveCount(0);
 await expect(page.getByRole('dialog').getByLabel('Notebook Quantity')).toHaveValue('1.000');
});

async function searchLookup(page:Page, name:string, query:string){
 await page.getByRole('button',{name,exact:true}).click();
 const dialog=page.getByRole('dialog').last();
 await dialog.getByRole('searchbox').fill(query);
 await expect(dialog.getByRole('status')).toHaveCount(0);
 await dialog.getByRole('button',{name:/^Close Choose /}).click();
}

test('configuration dialogs preserve errors, close after saves and discard calendar edits',async({page})=>{
 await seedProduct(page);
 await navigate(page, 'Unit conversions');
 await expect(page.getByLabel('Alternate unit',{exact:true})).toHaveCount(0);
 await page.getByRole('button',{name:'+ Add conversion',exact:true}).click();
 const conversion=page.getByRole('dialog',{name:'Add unit conversion',exact:true});
 await conversion.getByLabel('Conversion product',{exact:true}).selectOption('1');
 await conversion.getByLabel('Alternate unit',{exact:true}).fill('BOX');
 await conversion.getByLabel('Base units per alternate unit').fill('0');
 await conversion.getByRole('button',{name:'Add conversion',exact:true}).click();
 await expect(conversion.getByRole('alert')).toBeVisible();
 await expect(conversion.getByLabel('Alternate unit',{exact:true})).toHaveValue('BOX');
 await conversion.getByLabel('Base units per alternate unit').fill('12');
 await conversion.getByRole('button',{name:'Add conversion',exact:true}).click();
 await expect(conversion).toHaveCount(0);
 await expect(page.getByRole('cell',{name:'12.000 PCS',exact:true})).toBeVisible();
 await navigate(page, 'Catalog settings');
 await page.getByRole('button',{name:'Change financial year',exact:true}).click();
 await page.getByLabel('Year starting month').selectOption('1');
 page.once('dialog',d=>d.accept());await page.keyboard.press('Escape');
 await expect(page.getByText('Starting month:',{exact:false})).toHaveText('Starting month: April');
 await page.getByRole('button',{name:'+ Create group',exact:true}).click();
 await page.getByLabel('New group name').fill('Stationery');
 await page.getByRole('button',{name:'Create variant group',exact:true}).click();
 await expect(page.getByRole('dialog')).toHaveCount(0);
 await navigate(page, 'Expenses');
 await page.getByRole('button',{name:'New recurring expense',exact:true}).click();
 const expense=page.getByRole('dialog',{name:'New recurring expense',exact:true});
 await expense.getByLabel('Recurring description').fill('Monthly rent');
 await expense.getByLabel('Recurring amount ₹').fill('1000');
 await expense.getByRole('button',{name:'Save recurring expense'}).click();
 await expect(expense).toHaveCount(0);
 await expect(page.getByRole('cell',{name:'Monthly rent',exact:true})).toBeVisible();
});

test('offline help switches Hindi and Gujarati, remembers language and preserves business data',async({page})=>{
 await seedProduct(page);
 await navigate(page, 'Help');
 await expect(page.getByRole('heading',{name:'Help & getting started'})).toBeVisible();
 await page.getByLabel('Search help',{exact:true}).fill('backup');
 await expect(page.getByRole('heading',{name:'Back up, restore and restart',exact:true})).toBeVisible();
 await page.getByLabel('Language / भाषा / ભાષા').selectOption('hi');
 await expect(page.locator('html')).toHaveAttribute('lang','hi');
 await expect(page.getByRole('heading',{name:'सहायता और शुरुआत'})).toBeVisible();
 await page.getByLabel('सहायता खोजें',{exact:true}).fill('');
 await page.getByRole('navigation',{name:'सहायता के विषय'}).getByRole('button',{name:'अपना व्यवसाय सेट करें',exact:true}).click();
 await expect(page.getByRole('heading',{name:'अपना व्यवसाय सेट करें',exact:true})).toBeVisible();
 await page.getByRole('button',{name:'खोलें उत्पाद',exact:true}).click();
 await expect(page.getByRole('cell',{name:'Notebook',exact:true})).toBeVisible();
 await page.getByRole('button',{name:'+ जोड़ें उत्पाद',exact:true}).click();
 await page.getByRole('dialog').getByLabel('नाम',{exact:true}).fill('हिन्दी सेवा');
 await page.getByRole('dialog').getByLabel('SKU',{exact:true}).fill('HI-SERVICE');
 await page.getByRole('dialog').getByLabel('प्रकार',{exact:true}).selectOption('SERVICE');
 await page.getByRole('button',{name:'रिकॉर्ड सहेजें',exact:true}).click();
 const saved=await (await page.request.get(origin+'/api/products?business_id=1&q=HI-SERVICE')).json();
 expect(saved[0].kind).toBe('SERVICE');expect(saved[0].name).toBe('हिन्दी सेवा');
 await page.getByLabel('Language / भाषा / ભાષા').selectOption('gu');
 await expect(page.getByRole('heading',{name:'ઉત્પાદનો',exact:true})).toBeVisible();
 await page.reload();
 await expect(page.getByLabel('Language / भाषा / ભાષા')).toHaveValue('gu');
 await expect(page.locator('html')).toHaveAttribute('lang','gu');
 await navigate(page, 'મદદ');
 await expect(page.getByRole('heading',{name:'મદદ અને શરૂઆત'})).toBeVisible();
 await page.getByLabel('મદદ શોધો',{exact:true}).fill('સ્ટોક');
 await expect(page.getByRole('navigation',{name:'મદદના વિષયો'}).getByRole('button',{name:'સ્ટોક મેળવો અને એકમો ગોઠવો',exact:true})).toBeVisible();
 await page.getByLabel('Language / भाषा / ભાષા').selectOption('en');
 const products=await (await page.request.get(origin+'/api/products?business_id=1&q=BOOK')).json();
 expect(products[0].name).toBe('Notebook');expect(products[0].sale_price).toBe(10000);expect(products[0].kind).toBe('PRODUCT');
});

test('language selection is available at login and account roles keep help access',async({page})=>{
 await post(page,'auth/users',{username:'admin',password:'Admin-Password-123!',role:'ADMIN',active:1});
 await post(page,'auth/users',{username:'viewer',password:'Viewer-Password-123!',role:'VIEWER',active:1});
 await post(page,'auth/logout',{});await page.reload();
 await expect(page.getByRole('heading',{name:'Sign in to Ledgerleaf',exact:true})).toBeVisible();
 await page.getByLabel('Language / भाषा / ભાષા').selectOption('hi');
 await expect(page.getByLabel('Language / भाषा / ભાષા')).toHaveValue('hi');
 await page.getByLabel('उपयोगकर्ता नाम',{exact:true}).fill('viewer');
 await page.getByLabel('पासवर्ड',{exact:true}).fill('Viewer-Password-123!');
 await page.getByRole('button',{name:'साइन इन करें',exact:true}).click();
 await navigate(page, 'सहायता');
 await expect(page.getByRole('heading',{name:'सहायता और शुरुआत'})).toBeVisible();
 await page.getByLabel('सहायता खोजें',{exact:true}).fill('backup');
 await expect(page.getByRole('button',{name:'खोलें बैकअप और पुनर्स्थापना',exact:true})).toHaveCount(0);
});

test('database workbook and blank import templates download from their pages', async ({page}) => {
  await navigate(page, 'Backup & restore');
  await expect(page.getByRole('heading', {name: 'Entire database as Excel'})).toBeVisible();
  const workbookPromise = page.waitForEvent('download');
  await page.getByRole('link', {name: 'Download database workbook (.xlsx)'}).click();
  const workbook = await workbookPromise;
  expect(workbook.suggestedFilename()).toBe('Ledgerleaf-entire-database.xlsx');
  expect(await workbook.failure()).toBeNull();
  await navigate(page, 'Import & export');
  const templatePromise = page.waitForEvent('download');
  await page.getByRole('link', {name: 'Download blank products Excel template'}).click();
  const template = await templatePromise;
  expect(template.suggestedFilename()).toBe('Ledgerleaf-products-template.xlsx');
  expect(await template.failure()).toBeNull();
  await page.getByLabel('Record type', {exact:false}).selectOption('parties');
  await expect(page.getByRole('link', {name:'Download blank parties Excel template'})).toHaveAttribute('href','/api/import-template?entity=parties');
});

test('account assignment reaches business 201 and preserves hidden selections when edited',async({page})=>{
 await page.evaluate(async()=>{for(let n=2;n<=201;n++){const r=await fetch('/api/businesses',{method:'POST',headers:{'Content-Type':'application/json','X-Ledgerleaf':'1'},body:JSON.stringify({name:`Branch ${String(n).padStart(3,'0')}`,state_code:'27'})});if(!r.ok)throw new Error(await r.text())}});
 await post(page,'auth/users',{username:'admin',password:'Admin-Password-123!',role:'ADMIN',active:1});
 await page.reload();
 await navigate(page, 'Users & permissions');
 await page.getByRole('button',{name:'Add account',exact:true}).click();
 const dialog=page.getByRole('dialog',{name:'Add account',exact:true});
 await dialog.getByLabel('Username',{exact:true}).fill('branches');
 await dialog.getByLabel('Password',{exact:true}).fill('Branch-Password-123!');
 await dialog.getByLabel('Role',{exact:true}).selectOption('VIEWER');
 await dialog.getByLabel('Limit access to selected businesses').check();
 await dialog.getByLabel('Find business',{exact:true}).fill('Test Shop');
 await dialog.getByRole('checkbox',{name:'Test Shop · #1',exact:true}).check();
 await dialog.getByLabel('Find business',{exact:true}).fill('Branch 201');
 await dialog.getByRole('checkbox',{name:'Branch 201 · #201',exact:true}).check();
 await dialog.getByRole('button',{name:'Save account'}).click();
 const row=page.getByRole('row').filter({has:page.getByRole('cell',{name:'branches',exact:true})});
 await expect(row).toContainText('2 businesses assigned');
 await row.getByRole('button',{name:'Edit account'}).click();
 const edit=page.getByRole('dialog',{name:'Edit account',exact:true});
 await edit.getByLabel('Find business',{exact:true}).fill('Branch 201');
 await expect(edit.getByRole('checkbox',{name:'Branch 201 · #201',exact:true})).toBeChecked();
 await edit.getByRole('button',{name:'Save account'}).click();
 const users=await (await page.request.get(origin+'/api/auth/users')).json();
 expect(JSON.parse(users.find((u:any)=>u.username==='branches').business_ids).sort((a:number,b:number)=>a-b)).toEqual([1,201]);
});

test('password change dialog retains errors and closes after success',async({page})=>{
 await post(page,'auth/users',{username:'admin',password:'Admin-Password-123!',role:'ADMIN',active:1});await page.reload();
 await navigate(page, 'Users & permissions');
 await page.getByRole('button',{name:'Change your password',exact:true}).click();
 const dialog=page.getByRole('dialog',{name:'Change your password',exact:true});
 await dialog.getByLabel('Current password').fill('Wrong-Password-123!');
 await dialog.getByLabel('New password',{exact:true}).fill('Changed-Password-123!');
 await dialog.getByRole('button',{name:'Change password',exact:true}).click();
 await expect(dialog.getByRole('alert')).toBeVisible();
 await dialog.getByLabel('Current password').fill('Admin-Password-123!');
 await dialog.getByRole('button',{name:'Change password',exact:true}).click();
 await expect(dialog).toHaveCount(0);
 await expect(page.getByRole('status')).toContainText('Password changed');
});

test('bank creation and statement matching use dialogs and preserve validation errors',async({page})=>{
 await navigate(page, 'Banks & periods');
 await expect(page.getByRole('textbox',{name:'Account code',exact:true})).toHaveCount(0);
 await page.getByRole('button',{name:'Add bank account',exact:true}).click();
 const create=page.getByRole('dialog',{name:'Add bank account',exact:true});
 await create.getByLabel('Account code',{exact:true}).fill('6100');
 await create.getByLabel('Bank name',{exact:true}).fill('Trading bank');
 await create.getByLabel('Account number',{exact:true}).fill('00123');
 await page.screenshot({path:resolve('../release/verification-bank-language/bank-dialog.png')});
 await create.getByRole('button',{name:'Create bank account',exact:true}).click();
 await expect(create).toHaveCount(0);
 await page.getByRole('button',{name:'Add bank account',exact:true}).click();
 await create.getByLabel('Account code',{exact:true}).fill('6100');
 await create.getByLabel('Bank name',{exact:true}).fill('Duplicate bank');
 await create.getByRole('button',{name:'Create bank account',exact:true}).click();
 await expect(create.getByRole('alert')).toBeVisible();
 await expect(create.getByLabel('Bank name',{exact:true})).toHaveValue('Duplicate bank');
 page.once('dialog',d=>d.accept());await create.getByRole('button',{name:'Close Add bank account',exact:true}).click();
 await navigate(page, 'Expenses');
 await page.getByLabel('Payment account',{exact:true}).selectOption('6100');
 await page.getByLabel('Amount (₹)',{exact:true}).fill('25');
 await page.getByLabel('Date',{exact:true}).fill('2026-10-16');
 await page.getByLabel('Description',{exact:true}).fill('Bank expense');
 await page.getByRole('button',{name:'Post expense',exact:true}).click();
 await expect(page.getByRole('status')).toContainText('Expense posted');
 await navigate(page, 'Banks & periods');
 await page.getByRole('row').filter({hasText:'Bank expense'}).getByRole('button',{name:'Match',exact:true}).click();
 const match=page.getByRole('dialog');
 await match.getByLabel('Statement date').fill('2026-10-16');
 await match.getByLabel('Statement reference').fill('Statement-001');
 await match.getByRole('button',{name:'Match statement entry'}).click();
 await expect(match).toHaveCount(0);
 await expect(page.getByRole('row').filter({hasText:'Bank expense'})).toContainText('Statement-001');
});

test('receipt and dispatch dialogs fit desktop and mobile layouts',async({page})=>{
 await navigate(page, 'Goods receipts');
 await expect(page.getByLabel('Receipt source',{exact:true})).toHaveCount(0);
 await page.getByRole('button',{name:'New goods receipt',exact:true}).click();
 await page.getByLabel('Receipt source',{exact:true}).selectOption('DIRECT');
 await page.screenshot({path:resolve('../release/verification-goods-dialogs/receipt-desktop.png')});
 await page.setViewportSize({width:390,height:844});
 await page.screenshot({path:resolve('../release/verification-goods-dialogs/receipt-mobile.png')});
 expect(await page.getByRole('dialog',{name:'New goods receipt',exact:true}).evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBeTruthy();
 page.once('dialog',d=>d.accept());await page.getByRole('button',{name:'Close New goods receipt',exact:true}).click();
 await navigate(page, 'Dispatch challans');
 await page.getByRole('button',{name:'New dispatch challan',exact:true}).click();
 await page.screenshot({path:resolve('../release/verification-goods-dialogs/dispatch-mobile.png')});
 expect(await page.getByRole('dialog',{name:'New dispatch challan',exact:true}).evaluate(el=>el.scrollWidth<=el.clientWidth+1)).toBeTruthy();
 await page.setViewportSize({width:1280,height:720});
 await page.screenshot({path:resolve('../release/verification-goods-dialogs/dispatch-desktop.png')});
});
