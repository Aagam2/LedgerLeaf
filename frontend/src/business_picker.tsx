import {t} from './i18n';
import React,{useEffect,useState} from 'react';
import {api} from './api';
export function BusinessPicker({value,onChange}:{value:any;onChange:(business:any)=>void}){
 const[query,setQuery]=useState(''),[page,setPage]=useState(1),[rows,setRows]=useState<any[]>([]),[loading,setLoading]=useState(true),[error,setError]=useState('');
 useEffect(()=>{let live=true;setLoading(true);setError('');api(`businesses?paged=1&page=${page}&q=${encodeURIComponent(query)}`).then(r=>{if(live)setRows(r)}).catch(e=>{if(live){setRows([]);setError(String(e))}}).finally(()=>{if(live)setLoading(false)});return()=>{live=false}},[query,page,value?.id,value?.name]);
 return <><label>{t("Find business")}<input maxLength={160} value={query} onChange={e=>{setQuery(e.target.value);setPage(1)}}/></label><select aria-label={t("Current business")} disabled={loading} value={value?.id||''} onChange={e=>onChange(rows.find(b=>b.id===Number(e.target.value))||null)}><option value="">{t("Select a business")}</option>{value&&!rows.some(b=>b.id===value.id)&&<option value={value.id}>{value.name}</option>}{rows.map(b=><option key={b.id} value={b.id}>{b.name}</option>)}</select><div className="pagination"><button disabled={loading||page===1} onClick={()=>setPage(p=>p-1)}>{t("Previous businesses")}</button><span>{page}</span><button disabled={loading||rows.length<50} onClick={()=>setPage(p=>p+1)}>{t("Next businesses")}</button></div>{error&&<p role="alert">{error}</p>}</>
}
