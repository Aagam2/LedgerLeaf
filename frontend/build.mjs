import {build} from 'vite';

// This local application has no environment-based frontend configuration.
// Explicitly disable Vite's automatic .env loading, including production builds.
await build({envFile: false});
