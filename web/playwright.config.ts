import { defineConfig } from '@playwright/test';
import { existsSync, readdirSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';
const cache=join(homedir(),'.cache','ms-playwright');
const candidates=existsSync(cache)?readdirSync(cache).filter(n=>/^chromium-/.test(n)).sort().reverse().map(n=>join(cache,n,'chrome-linux64','chrome')):[];
const executablePath=process.env.HILIFE_CHROMIUM??candidates.find(p=>existsSync(p));
export default defineConfig({testDir:'tests',workers:1,fullyParallel:false,use:{baseURL:'http://127.0.0.1:8787',viewport:{width:1440,height:1000},launchOptions:{executablePath,args:['--no-sandbox']}},webServer:{command:'../bin/hilife web --listen 127.0.0.1:8787',url:'http://127.0.0.1:8787/api/state',reuseExistingServer:true},reporter:'list'});
