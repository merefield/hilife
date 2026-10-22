import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { writeFileSync } from 'node:fs';
export default defineConfig({plugins:[svelte(),{name:'preserve-go-embed-directory',writeBundle(){writeFileSync(new URL('../internal/webui/dist/.gitkeep',import.meta.url),'');}}],build:{outDir:'../internal/webui/dist',emptyOutDir:true},server:{proxy:{'/api':{target:'http://127.0.0.1:8787',configure(proxy){proxy.on('proxyReq',req=>req.removeHeader('origin'));}}}}});
