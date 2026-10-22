import {t} from './i18n';
import React,{useEffect,useState} from 'react';
import {api} from './api';
import {displayScaled} from './numbers';

export function PaymentHistory({businessId,documentId}:{businessId:number;documentId:number}){
  const[rows,setRows]=useState<any[]>([]),[page,setPage]=useState(1),[loading,setLoading]=useState(true),[error,setError]=useState('');
  useEffect(()=>{let live=true;setLoading(true);setError('');api(`payments?business_id=${businessId}&document_id=${documentId}&page=${page}`).then(r=>{if(live)setRows(r)}).catch(e=>{if(live)setError(String(e))}).finally(()=>{if(live)setLoading(false)});return()=>{live=false}},[businessId,documentId,page]);
  return <section aria-label={t("Payment history")}><h3>{t("Recorded payments")}</h3><p>Direct payments recorded against this bill. Advance allocations and return refunds are shown in Party balances and Credit notes.</p>{error&&<p className="error" role="alert">{error}</p>}{loading?<p>Loading payments…</p>:<><table><thead><tr><th>Payment ID</th><th>{t("Date")}</th><th>{t("Mode")}</th><th>{t("Account code")}</th><th>{t("Amount")}</th></tr></thead><tbody>{rows.map(r=><tr key={r.id}><td>{r.id}</td><td>{r.date}</td><td>{r.mode}</td><td>{r.account_code||'Legacy payment'}</td><td>₹ {displayScaled(r.amount,2)}</td></tr>)}</tbody></table>{!rows.length&&<p>No direct payments on this page.</p>}</>}<div className="pagination"><button disabled={loading||page===1} onClick={()=>setPage(page-1)}>{t("Previous payments")}</button><span>{t("Page")} {page}</span><button disabled={loading||rows.length<50} onClick={()=>setPage(page+1)}>{t("Next payments")}</button></div></section>
}
