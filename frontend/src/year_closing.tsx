import {Modal} from './modal';
import {t} from './i18n';
import React, {useEffect,useState} from 'react';
import {api} from './api';
import {displayScaled} from './numbers';

type Closing = {date:string;net_profit:number;reference:string};
type Preview = {date:string;net_profit:number;lines:{Account:string;Debit:number;Credit:number}[]};
export function YearClosing({businessId,onClosed}:{businessId:number;onClosed:()=>void}) {
 const [date,setDate]=useState(''),[rows,setRows]=useState<Closing[]>([]),[preview,setPreview]=useState<Preview|null>(null);
 const [error,setError]=useState(''),[busy,setBusy]=useState(false),[open,setOpen]=useState(false),[message,setMessage]=useState('');
 async function load(){setRows(await api(`year-close?business_id=${businessId}`));}
 useEffect(()=>{let live=true;api(`year-close?business_id=${businessId}`).then(r=>{if(live)setRows(r)}).catch(e=>{if(live)setError(String(e))});return()=>{live=false}},[businessId]);
 async function review(){
  setBusy(true);setError('');setPreview(null);
  try{setPreview(await api(`year-close?business_id=${businessId}`,{date,preview:true}));}catch(e){setError(String(e));}finally{setBusy(false);}
 }
 async function close(){
  if(!preview||preview.date!==date||busy)return;
  if(!confirm(`Close the financial year ending ${date}? Income and expense balances move to retained earnings, and postings through this date are permanently locked. Balances are recalculated when posted. Verify reports and create a backup first.`))return;
  setBusy(true);setError('');
  try{await api(`year-close?business_id=${businessId}`,{date,confirm:'CLOSE FINANCIAL YEAR'});setPreview(null);setOpen(false);setMessage(`Financial year ending ${date} closed.`);onClosed();await load();}
  catch(e){setError(String(e));setPreview(null);}finally{setBusy(false);}
 }
 return <section className="card"><h2>Financial-year closing</h2>
  <p>Transfer income and expense balances to retained earnings (3100). Profit-and-loss reports exclude closing entries; the ledger and balance sheet retain them. Closing is permanent; verify your reports and create a backup first.</p>
  {error&&<p role="alert">{error}</p>}
  {message&&<p role="status">{message}</p>}
  <button disabled={busy} onClick={()=>{setDate('');setPreview(null);setError('');setMessage('');setOpen(true)}}>Review year closing</button>
  {open&&<Modal title="Financial-year closing" busy={busy} onClose={()=>setOpen(false)}>
  {error&&<p className="error" role="alert">{error}</p>}
  <p>Closing permanently locks financial postings through the selected date. Review reports and create a backup before confirming.</p>
  <label>Financial year-end date<input type="date" disabled={busy} value={date} onChange={e=>{setDate(e.target.value);setPreview(null);setError('');}}/></label>
  <button disabled={busy||!date} onClick={review}>Preview year closing</button>
  {preview&&<><p role="status">{preview.net_profit<0?'Loss':'Profit'} to transfer: ₹ {displayScaled(Math.abs(preview.net_profit),2)}. No closing entries have been saved.</p>
   {preview.lines.length>0?<div className="table-wrap"><table><thead><tr><th>{t("Account")}</th><th>Debit</th><th>Credit</th></tr></thead><tbody>{preview.lines.map(l=><tr key={l.Account}><td>{l.Account}</td><td>₹ {displayScaled(l.Debit,2)}</td><td>₹ {displayScaled(l.Credit,2)}</td></tr>)}</tbody></table></div>:<p>There are no income or expense balances to transfer. Closing will still lock postings through this date.</p>}
   <p>This preview does not reserve balances. Closing recalculates them from the latest posted entries.</p>
  </>}
  <button disabled={busy||!preview||preview.date!==date} onClick={close}>Close financial year</button>
  <button type="button" disabled={busy} onClick={()=>setOpen(false)}>{t('Cancel')}</button>
  </Modal>}
  <h3>Closed financial years</h3>
  {!rows.length&&<p>No financial years have been closed.</p>}
  <ul>{rows.map(r=><li key={r.date}>{r.date} · Net profit transferred ₹ {displayScaled(r.net_profit,2)}</li>)}</ul>
 </section>;
}
