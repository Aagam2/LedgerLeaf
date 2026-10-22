import {defineConfig} from '@playwright/test';
export default defineConfig({testDir:'../tests/e2e',workers:1,timeout:45000,outputDir:'../release/test-results',use:{screenshot:'only-on-failure',headless:true,launchOptions:{executablePath:process.env.CHROME_PATH||'/usr/bin/google-chrome',args:['--no-sandbox']}}});
