import React,{useEffect,useState} from 'react';
import {api} from './api';

export function PaymentAccount({businessId,label,initial='1000',disabled=false}:{businessId:number;label:string;initial?:string;disabled?:boolean}){
 const[banks,setBanks]=useState<any[]>([]),[loading,setLoading]=useState(true),[error,setError]=useState('');
 useEffect(()=>{let live=true;setLoading(true);setError('');api(`banks?business_id=${businessId}`).then(r=>{if(live)setBanks(r)}).catch(e=>{if(live)setError(String(e))}).finally(()=>{if(live)setLoading(false)});return()=>{live=false}},[businessId]);
 return <><label>{label}<select aria-label={label} name="account" defaultValue={initial} disabled={disabled||loading||!!error} required><option value="1000">Cash</option>{!banks.some(b=>b.code===initial)&&initial!=='1000'&&<option value={initial}>Account {initial}</option>}{banks.map(b=><option key={b.code} value={b.code}>{b.name} · {b.code}</option>)}</select></label>{error&&<p role="alert">Payment accounts could not be loaded: {error}</p>}</>
}
