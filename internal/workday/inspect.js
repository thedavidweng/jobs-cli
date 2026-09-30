(bindings) => {
 const visible = e => !!(e.getClientRects().length) && !e.closest('[hidden],[aria-hidden="true"]');
 const text = e => e?.textContent.trim() || '';
 const selector = e => e.id ? '#'+CSS.escape(e.id) : '[data-automation-id='+JSON.stringify(e.getAttribute('data-automation-id'))+']';
 const {names,sections:sectionIDs,rows:rowIDs,keys}=bindings;
 const label = e => text(e.labels?.[0]) || e.getAttribute('aria-label') || text(document.getElementById(e.getAttribute('aria-labelledby'))) || e.name || e.id;
 const controls = {},values = {},fields = [],questions = [],sections = [];
 let pending = '';
 const page = [...document.querySelectorAll('[data-automation-id="applyFlowPage"]')].find(visible);
 const step = text(page);
 const confirmation = text(document.querySelector('[data-automation-id="applicationConfirmation"],[data-automation-id="thankYouMessage"]'));
 const receipt = /application (submitted|received)|thank you for (applying|your application)/i.test(confirmation)?confirmation:'';
 if (document.querySelector('[data-automation-id="signInContent"],[data-automation-id="signInForm"]')) pending='login_required';
 else if (/verify your email|verification code/i.test(text(document.body))) pending='verification_required';
 else if (document.querySelector('iframe[src*="captcha"],[data-automation-id="assessment"]')) pending='user_action_required';
 else if (!page && !receipt) pending = document.querySelector('[data-automation-id="adventureButton"]') ? 'start_application_required' : 'unsupported_step';
 for (const [name,automation] of Object.entries(sectionIDs)) {
  const section = document.querySelector('[data-automation-id='+JSON.stringify(automation)+']');
  if (!section || !visible(section)) continue;
  const rows = [...section.querySelectorAll('[data-automation-id="'+rowIDs[name]+'"]')];
  const sectionFields = [];
  for (const e of rows[0]?.querySelectorAll('input,textarea,select') || []) {
   if (!visible(e)) continue;
   const id = e.getAttribute('data-automation-id') || e.name;
   if(!keys[id]) {pending='unsupported_step';continue;}
   sectionFields.push({id:keys[id],label:label(e),type:e.tagName==='SELECT'?'multi_value_single_select':['month','date'].includes(e.type)?e.type:'input_text',required:e.required || e.getAttribute('aria-required')==='true',options:[...e.options||[]].filter(o=>o.value).map(o=>({value:o.value,label:o.textContent.trim()}))});
  }
  sections.push({name,fields:sectionFields,rows:rows.map(row=>Object.fromEntries([...row.querySelectorAll('input,textarea,select')].map(e=>[e.getAttribute('data-automation-id')||e.name,e.value])))});
 }
 for (const e of document.querySelectorAll('input,textarea,select,[role="combobox"]')) {
  if (!visible(e) || e.type==='hidden' || e.type==='password' || e.closest('[data-automation-id$="Section"]')) continue;
  const automation=e.getAttribute('data-automation-id');
  if (!e.id && !automation && !e.name) {pending='unsupported_step';continue;}
  const name=names[automation];
  const id=e.name || e.id || automation;
  if(e.type==='radio') {
   if(questions.some(q=>q.id===id))continue;
   const group=[...document.querySelectorAll('input[type="radio"]')].filter(r=>visible(r) && (r.name||r.id||r.getAttribute('data-automation-id'))===id);
   questions.push({id,label:text(e.closest('fieldset')?.querySelector('legend'))||label(e),type:'multi_value_single_select',required:group.some(r=>r.required || r.getAttribute('aria-required')==='true'),options:group.map(r=>({value:r.value,label:label(r)}))});
   values[id]=group.find(r=>r.checked)?.value||'';
   continue;
  }
  let type=e.type==='file'?'input_file':e.tagName==='SELECT'?'multi_value_single_select':e.type==='checkbox'?'boolean':e.type==='radio'?'multi_value_single_select':'input_text';
  const options=[...e.options||[]].filter(o=>o.value).map(o=>({value:o.value,label:o.textContent.trim()}));
  if (e.getAttribute('role')==='combobox' && e.tagName!=='SELECT') {pending='unsupported_step';}
  values[id]=e.type==='checkbox'?String(e.checked):e.value;
  controls[id]=selector(e);
  if (name && type!=='boolean') {
   fields.push({name,label:label(e),options,type:e.type==='file'?'file':type,required:e.required || e.getAttribute('aria-required')==='true'});
   controls['field:'+name]=selector(e);
   if (e.type==='file') {controls['file:'+name]=selector(e);values['file:'+name]=[...e.files].map(f=>f.name).join(',');}
  } else {
   if(e.type==='file')pending='unsupported_step';
   questions.push({id,label:label(e),type,required:e.required || e.getAttribute('aria-required')==='true',options});
  }
 }
 for (const [key,ids] of Object.entries({start:['adventureButton'],next:['bottom-navigation-next-button'],submit:['submitButton','bottom-navigation-submit-button']})) {
  const buttons=[...document.querySelectorAll('button')].filter(e=>visible(e) && !e.disabled && ids.includes(e.getAttribute('data-automation-id')));
  if (buttons.length===1) controls[key]=selector(buttons[0]);
 }
 const tasks=[...document.querySelectorAll('[data-automation-id="candidateHomeTask"]')].map(text);
 return {url:location.href,step,pending,fields,questions,sections,controls,values,review:controls.submit?text(document.body):'',receipt,application_id:text(document.querySelector('[data-automation-id="applicationId"]')),tasks};
}
