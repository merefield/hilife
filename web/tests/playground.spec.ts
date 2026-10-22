import { test,expect } from '@playwright/test';
test.beforeEach(async({page,request})=>{await request.post('/api/action',{data:{type:'pause'}});const s=await(await request.get('/api/state')).json();if(s.showTrails)await request.post('/api/action',{data:{type:'trails'}});if(s.wrap)await request.post('/api/action',{data:{type:'wrap'}});await request.post('/api/action',{data:{type:'pattern',value:'glider'}});await page.goto('/');await expect(page.getByText('LIVE ENGINE',{exact:true})).toBeVisible();});
test('playback, undo, reset and million-generation HashLife jump',async({page,request})=>{
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message));
 await page.getByRole('button',{name:'Step',exact:false}).click();await expect.poll(async()=> (await(await request.get('/api/state')).json()).generation).toBe(1);
 await page.getByRole('button',{name:'Undo',exact:true}).click();await expect.poll(async()=> (await(await request.get('/api/state')).json()).generation).toBe(0);
 await page.getByRole('button',{name:'Run simulation'}).click();await expect(page.getByRole('button',{name:'Pause simulation'})).toBeVisible();await expect.poll(async()=> (await(await request.get('/api/state')).json()).generation).toBeGreaterThan(1);await page.getByRole('button',{name:'Pause simulation'}).click();
 await page.getByRole('button',{name:'Reset pattern'}).click();await expect.poll(async()=> (await(await request.get('/api/state')).json()).generation).toBe(0);
 await page.getByLabel('Fast-forward generations').selectOption('1048576');await page.getByRole('button',{name:'Jump ↗',exact:true}).click();await expect.poll(async()=> (await(await request.get('/api/state')).json()).generation).toBe(1048576);expect(errors).toEqual([]);
});
test('mouse editing, patterns, themes and responsive layout',async({page,request})=>{
 await page.getByRole('button',{name:'Clear board'}).click();await expect.poll(async()=> (await(await request.get('/api/state')).json()).population).toBe(0);
 const board=page.locator('.field > canvas');const box=(await board.boundingBox())!;await page.mouse.click(box.x+box.width/2,box.y+box.height/2);await expect.poll(async()=> (await(await request.get('/api/state')).json()).population).toBe(1);await page.mouse.click(box.x+box.width/2,box.y+box.height/2,{button:'right'});await expect.poll(async()=> (await(await request.get('/api/state')).json()).population).toBe(0);
 await page.getByRole('button',{name:'Glider gun',exact:true}).click();await page.getByLabel('neon theme').click();await expect.poll(async()=> (await(await request.get('/api/state')).json()).theme).toBe('neon');await page.getByLabel('Fading trails',{exact:true}).click();await expect.poll(async()=> (await(await request.get('/api/state')).json()).showTrails).toBe(true);
 await page.screenshot({path:'../artifacts/hilife-desktop.png',fullPage:true});
 await page.setViewportSize({width:390,height:844});await expect(page.getByRole('button',{name:'Run simulation'})).toBeVisible();expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBe(390);await page.screenshot({path:'../artifacts/hilife-mobile.png',fullPage:true});
});
test('imports, custom rules, exports and all eight automata',async({page,request})=>{
 await page.getByRole('button',{name:'↥ Import',exact:true}).click();await page.getByLabel('Pattern text').fill('x = 3, y = 3, rule = B36/S23\nbo$2bo$3o!');await page.getByRole('button',{name:'Plant pattern ↗'}).click();await expect.poll(async()=> (await(await request.get('/api/state')).json()).rule).toBe('B36/S23');await expect(page.getByRole('dialog')).not.toBeVisible();
 await page.getByLabel('Birth / survival').fill('B3/S23');await page.getByLabel('Birth / survival').press('Enter');await expect.poll(async()=> (await(await request.get('/api/state')).json()).rule).toBe('B3/S23');
 await page.getByRole('button',{name:'↧ Export'}).focus();const download=page.waitForEvent('download');await page.getByRole('link',{name:'Pattern · RLE'}).click();expect((await download).suggestedFilename()).toBe('hilife.rle');
 for(const mode of ['brians-brain','langtons-ant','larger-than-life','faders','forest-fire','wildfire','belousov','life']){await page.getByLabel('Automaton',{exact:true}).selectOption(mode);await expect.poll(async()=> (await(await request.get('/api/state')).json()).mode).toBe(mode);await page.getByRole('button',{name:'Step',exact:false}).click();}
});
