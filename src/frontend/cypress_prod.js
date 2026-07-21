import { defineConfig } from "cypress";

// WAKECI_E2E_PORT lets `make testprod` point these tests at an isolated,
// throwaway wakeci instance instead of the default dev/prod port.
const port = process.env.WAKECI_E2E_PORT || 8081;

export default defineConfig({
    video: false,
    e2e: {
        baseUrl: `http://localhost:${port}`,
        supportFile: "cypress/support/index.js",
        specPattern: "cypress/integration/*.js",
    },
});
