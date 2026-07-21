import { defineConfig } from "cypress";

export default defineConfig({
    video: false,
    // See cypress_prod.js: builds triggered mid-test need more than the
    // default 4000ms to reach "running" under load.
    defaultCommandTimeout: 10000,
    e2e: {
        baseUrl: "http://localhost:8080",
        supportFile: "cypress/support/index.js",
        specPattern: "cypress/integration/*.js",
    },
});
