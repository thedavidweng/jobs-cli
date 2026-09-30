(() => {
 const visible = e => !!(e.getClientRects().length) && !e.closest('[hidden],[aria-hidden="true"]');
 const text = e => e?.textContent.trim() || '';
 const selector = e => e.id ? '#'+CSS.escape(e.id) : '[data-automation-id='+JSON.stringify(e.getAttribute('data-automation-id'))+']';
 const names = {firstName:'first_name',lastName:'last_name',email:'email',emailAddress:'email',phoneNumber:'phone',addressLine1:'address_line',city:'city',postalCode:'postal_code',country:'country',countryRegion:'region',resume:'resume',coverLetter:'cover_letter'};
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
 for (const [name,automation] of Object.entries({work:'workExperienceSection',education:'educationSection',skills:'skillsSection',languages:'languagesSection',certificates:'certificationsSection'})) {
  const section = document.querySelector('[data-automation-id='+JSON.stringify(automation)+']');
  if (!section || !visible(section)) continue;
  const rows = [...section.querySelectorAll('[data-automation-id="'+({work:'workExperience',education:'education',skills:'skill',languages:'language',certificates:'certification'}[name])+'"]')];
  const sectionFields = [];
  const keys={company:"name",jobTitle:"position",startDate:"startDate",endDate:"endDate",description:"summary",school:"institution",degree:"studyType",fieldOfStudy:"area",skill:"name",language:"language",fluency:"fluency",certification:"name"};
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
  if (!e.id && !automation) {pending='unsupported_step';continue;}
  const name=names[automation];
  const id=e.name || e.id || automation;
  let type=e.type==='file'?'input_file':e.tagName==='SELECT'?'multi_value_single_select':e.type==='checkbox'?'boolean':e.type==='radio'?'multi_value_single_select':'input_text';
  const options=[...e.options||[]].filter(o=>o.value).map(o=>({value:o.value,label:o.textContent.trim()}));
  if (e.getAttribute('role')==='combobox' && e.tagName!=='SELECT') {pending='unsupported_step';}
  values[id]=e.type==='checkbox'?String(e.checked):e.value;
  controls[id]=selector(e);
  if (name && options.length===0 && type!=='boolean') {
   fields.push({name,label:label(e),type:e.type==='file'?'file':type,required:e.required || e.getAttribute('aria-required')==='true'});
   controls['field:'+name]=selector(e);
   if (e.type==='file') {controls['file:'+name]=selector(e);values['file:'+name]=[...e.files].map(f=>f.name).join(',');}
  } else {
   questions.push({id,label:label(e),type,required:e.required || e.getAttribute('aria-required')==='true',options});
  }
 }
 for (const [key,ids] of Object.entries({start:['adventureButton'],next:['bottom-navigation-next-button'],submit:['submitButton','bottom-navigation-submit-button']})) {
  const buttons=[...document.querySelectorAll('button')].filter(e=>visible(e) && !e.disabled && ids.includes(e.getAttribute('data-automation-id')));
  if (buttons.length===1) controls[key]=selector(buttons[0]);
 }
 const tasks=[...document.querySelectorAll('[data-automation-id="candidateHomeTask"]')].map(text);
 return {url:location.href,step,pending,fields,questions,sections,controls,values,review:controls.submit?text(document.body):'',receipt,application_id:text(document.querySelector('[data-automation-id="applicationId"]')),tasks};
})()
