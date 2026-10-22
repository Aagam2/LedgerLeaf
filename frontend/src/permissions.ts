import {createContext,useContext} from 'react';
export type Role='ADMIN'|'ACCOUNTANT'|'BILLER'|'VIEWER';
export const RoleContext=createContext<Role>('VIEWER');
export const useRole=()=>useContext(RoleContext);
export function visiblePage(role:Role,page:string):boolean {
 if(page==='Help'||page==='Users & permissions')return true;
 if(role==='ADMIN')return true;
 if(['Businesses','Backup & restore'].includes(page))return false;
 if(role==='BILLER')return ['Billing','POS','Parties','Products','Unit conversions','Warehouses'].includes(page);
 if(role==='VIEWER')return ['Overview','Billing','Commercial documents','Credit notes','Parties','Products','Unit conversions','Warehouses','Inventory','Stock history','Reports','Trial balance','Journals'].includes(page);
 return true;
}
