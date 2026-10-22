import {Modal} from './modal';
import {t} from './i18n';
import React,{useEffect,useState} from 'react';
import {api} from './api';
import {scaled,displayScaled} from './numbers';
import {localDate,requestKey} from './operations';
type Balance={product_id:number;name:string;unit:string;quantity:number;remaining:number;cost:number;used_cost:number};
export function DispatchSelection({businessId,id,mode,onDone,onCancel}:{businessId:number;id:number;mode:'invoice'|'return'|'history';onDone:(message:string)=>void;onCancel:()=>void}){
 const[historyPage,setHistoryPage]=useState(1);const[document,setDocument]=useState<any>(null),[balances,setBalances]=useState<Balance[]>([]),[quantities,setQuantities]=useState<Record<number,string>>({}),[history,setHistory]=useState<any[]>([]),[date,setDate]=useState(localDate()),[key,setKey]=useState(requestKey),[busy,setBusy]=useState(false),[loading,setLoading]=useState(true),[error,setError]=useState('');
 useEffect(()=>{let live=true;Promise.all([api(`dispatches?business_id=${businessId}&id=${id}`),api(`dispatches?business_id=${businessId}&id=${id}&remaining=1`)]).then(([d,b])=>{if(live){setDocument(d);setBalances(b);setQuantities(Object.fromEntries(b.map((l:Balance)=>[l.product_id,displayScaled(l.remaining,3)])))}}).catch(e=>{if(live)setError(String(e))}).finally(()=>{if(live)setLoading(false)});return()=>{live=false}},[businessId,id]);
 useEffect(()=>{let live=true;api(`dispatches?business_id=${businessId}&id=${id}&history=1&page=${historyPage}`).then(h=>{if(live)setHistory(h)}).catch(e=>{if(live)setError(String(e))});return()=>{live=false}},[businessId,id,historyPage]);
 async function submit(){
  setError('');
  try{
   const lines=balances.map(l=>({product_id:l.product_id,quantity:scaled(quantities[l.product_id]||'0',3)})).filter(l=>l.quantity>0);
   if(!lines.length)throw new Error('Select at least one positive quantity.');
   for(const l of lines){if(l.quantity>balances.find(b=>b.product_id===l.product_id)!.remaining)throw new Error('Quantity exceeds the remaining dispatch.');}
   if(!date)throw new Error('Select a date.');
   if(mode==='return'&&!confirm(`Return the selected quantities from dispatch #${id} to its warehouse on ${date}?`))return;
   setBusy(true);
   if(mode==='return'){
    await api(`dispatch-return?business_id=${businessId}`,{id,date,key,lines});
    onDone('Selected dispatched quantities returned. Remaining quantities are available for later invoicing or return.');
   }else{
    const b=await api(`documents?business_id=${businessId}`,{...document,id:0,dispatch_id:id,date,due_date:date,key,status:'DRAFT',document_type:'',lines:lines.map(l=>{const original=document.lines.find((x:any)=>x.product_id===l.product_id);return {...l,sale_unit:'',price:Math.round(original.price*1000/(original.unit_factor||1000)),discount:0}})});
    onDone(`Billing draft #${b.id} created. Review base-unit prices and taxes before finalizing. Drafts do not reserve dispatch quantities.`);
   }
  }catch(e){setError(String(e))}finally{setBusy(false)}
 }
 return <Modal title={`Dispatch #${id} · ${mode==='history'?'Allocation history':mode==='invoice'?'Choose invoice quantities':'Choose return quantities'}`} busy={busy} onClose={onCancel} confirmDiscard={mode!=='history'}>{error&&<p role="alert">{error}</p>}{loading?<p>Loading dispatch…</p>:<><p>Quantities are in base stock units. Invoiced quantities stay allocated even if the invoice is later returned; use Billing for returns of invoiced goods.</p><div className="table-wrap"><table><thead><tr><th>{t("Product")}</th><th>Unit</th><th>Dispatched</th><th>Remaining</th>{mode!=='history'&&<th>{t("Selected")}</th>}</tr></thead><tbody>{balances.map(l=><tr key={l.product_id}><td>{l.name}</td><td>{l.unit}</td><td>{displayScaled(l.quantity,3)}</td><td>{displayScaled(l.remaining,3)}</td>{mode!=='history'&&<td><input aria-label={`${l.name} selected dispatch quantity`} disabled={busy||l.remaining===0} value={quantities[l.product_id]||'0'} onChange={e=>{setQuantities({...quantities,[l.product_id]:e.target.value});setKey(requestKey())}}/></td>}</tr>)}</tbody></table></div>{mode!=='history'&&<><p>Set a quantity to zero to leave it for later. Creating an invoice draft does not move stock or reserve quantities. Stale drafts can fail when finalized.</p><label>{mode==='invoice'?'Invoice':'Return'} date<input type="date" disabled={busy} value={date} onChange={e=>{setDate(e.target.value);setKey(requestKey())}}/></label><button disabled={busy||!document||!balances.some(l=>l.remaining>0)} onClick={submit}>{mode==='invoice'?'Create partial invoice draft':'Return selected quantities'}</button></>}<h3>Posted allocations</h3>{history.length?<div className="table-wrap"><table><thead><tr><th>{t("Date")}</th><th>Action</th><th>{t("Product")}</th><th>Base quantity</th><th>Cost</th></tr></thead><tbody>{history.map(h=><tr key={`${h.reference}/${h.product_id}`}><td>{h.date}</td><td>{h.document_id?`Invoice #${h.document_id}`:'Warehouse return'}</td><td>{balances.find(b=>b.product_id===h.product_id)?.name||h.product_id}</td><td>{displayScaled(h.quantity,3)}</td><td>₹ {displayScaled(h.value,2)}</td></tr>)}</tbody></table></div>:<p>No allocations on this page.</p>}<div className="pagination"><button disabled={busy||historyPage===1} onClick={()=>setHistoryPage(p=>p-1)}>Previous allocations</button><span>Allocation page {historyPage}</span><button disabled={busy||history.length<50} onClick={()=>setHistoryPage(p=>p+1)}>Next allocations</button></div></>}<button disabled={busy} onClick={onCancel}>Close dispatch selection</button></Modal>
}
