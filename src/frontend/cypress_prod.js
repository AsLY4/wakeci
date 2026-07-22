import { defineConfig } from "cypress";

// WAKECI_E2E_PORT lets `make testprod` point these tests at an isolated,
// throwaway wakeci instance instead of the default dev/prod port.
const port = process.env.WAKECI_E2E_PORT || 8081;

export default defineConfig({
    video: false,
    expose: {
        wakeUrl: `http://localhost:${port}/`,
    },
    // Several specs poll for a build to reach "running"/"pending" right
    // after triggering it; the default 4000ms is tight enough to flake under
    // system load even though the app itself is behaving correctly.
    defaultCommandTimeout: 10000,
    e2e: {
        baseUrl: `http://localhost:${port}`,
        supportFile: "cypress/support/index.js",
        specPattern: "cypress/integration/*.js",
    },
});
