import {t} from './i18n';
import React, {useState} from 'react';
import {requestKey} from './operations';
import {displayScaled} from './numbers';

type PreviewEntry = {reference:string;date:string;description:string;lines:number;debit:number;credit:number};
export function JournalImport({businessId}:{businessId:number}) {
 const [file,setFile]=useState<File|null>(null),[key,setKey]=useState(requestKey);
 const [preview,setPreview]=useState(false),[entries,setEntries]=useState<PreviewEntry[]>([]),[page,setPage]=useState(1);
 const [busy,setBusy]=useState(false),[message,setMessage]=useState(''),[error,setError]=useState('');
 async function run(check:boolean) {
  if(!file||busy)return;
  if(!check&&(!preview||!confirm(`Post ${entries.length} previewed journals? They become immutable; corrections require journal reversals.`)))return;
  setBusy(true);setError('');setMessage('');setPreview(false);
  try {
   if(file.size>20*1024*1024)throw new Error('Maximum file size is 20 MB.');
   const format=file.name.toLowerCase().endsWith('.xlsx')?'xlsx':'csv';
   const r=await fetch(`/api/import-journals?business_id=${businessId}&key=${key}&format=${format}&preview=${check?'1':'0'}`,{method:'POST',headers:{'X-Ledgerleaf':'1','Content-Type':'application/octet-stream'},body:file});
   const result=await r.json();
   if(!r.ok)throw new Error(result.error||'Import failed');
   if(result.already_imported){setEntries([]);setMessage(`${result.journals} journals were already imported. No additional journals were posted.`);return;}
   setPreview(check);setEntries(check?result.entries:[]);setPage(1);
   setMessage(check?`${result.journals} balanced journals validated. Review the dates, references and totals below. No records were saved.`:`${result.journals} journals posted.`);
  } catch(e){setError(String(e));setEntries([]);} finally {setBusy(false);}
 }
 function template(){
  const url=URL.createObjectURL(new Blob(['reference,date,description,account,debit,credit\r\n'],{type:'text/csv;charset=utf-8'}));
  const link=document.createElement('a');link.href=url;link.download='journal-import-template.csv';link.click();setTimeout(()=>URL.revokeObjectURL(url),1000);
 }
 return <section className="card"><h2>Financial journal import</h2>
  <p>CSV/XLSX headers: reference,date,description,account,debit,credit. Use integer paise, one positive debit or credit per row, and zero on the other side. Use the same date (YYYY-MM-DD) and description for all rows in a reference. Every reference must balance. Maximum 20 MB, 10,000 lines, 1,000 journals and 100 lines per journal. References are unique within the business.</p>
  <p>Use this for cash, bank, opening-equity, expenses and custom accounts. Inventory, receivables, payables, tax and other operational control accounts require their normal workflows. Imported journals can be reversed in Journal entries.</p>
  <button type="button" onClick={template}>Download journal CSV template</button>
  {error&&<p role="alert">{error}</p>}{message&&<p role="status">{message}</p>}
  <label>Financial journal file<input type="file" accept=".csv,.xlsx" disabled={busy} onChange={e=>{setFile(e.target.files?.[0]||null);setPreview(false);setEntries([]);setPage(1);setKey(requestKey());setMessage('');setError('');}}/></label>
  <button disabled={busy||!file} onClick={()=>run(true)}>Validate financial import</button>
  <button disabled={busy||!preview} onClick={()=>run(false)}>Post financial import</button>
  {entries.length>0&&<><div className="table-wrap"><table><thead><tr><th>Reference</th><th>{t("Date")}</th><th>{t("Description")}</th><th>Lines</th><th>Debit</th><th>Credit</th></tr></thead><tbody>{entries.slice((page-1)*50,page*50).map(e=><tr key={e.reference}><td>{e.reference}</td><td>{e.date}</td><td>{e.description}</td><td>{e.lines}</td><td>₹ {displayScaled(e.debit,2)}</td><td>₹ {displayScaled(e.credit,2)}</td></tr>)}</tbody></table></div><div className="pagination"><button disabled={busy||page===1} onClick={()=>setPage(page-1)}>Previous preview</button><span>{t("Page")} {page} of {Math.ceil(entries.length/50)}</span><button disabled={busy||page*50>=entries.length} onClick={()=>setPage(page+1)}>Next preview</button></div><p>Posting validates the file again. New period locks or a duplicate reference can prevent posting; the entire import then rolls back.</p></>}
 </section>;
}
