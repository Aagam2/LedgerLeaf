import {Page} from '../../frontend/node_modules/@playwright/test';

// Expand the relevant menu group through the same control used by a person.
export async function navigate(page:Page,label:string){
 const menu=page.locator('nav.app-navigation');
 const button=menu.getByRole('button',{name:label,exact:true,includeHidden:true});
 const group=button.locator('xpath=ancestor::details[1]');
 if(await group.count()&&!(await group.evaluate(element=>(element as HTMLDetailsElement).open))){
  await group.locator(':scope > summary').click();
 }
 await button.click();
}
