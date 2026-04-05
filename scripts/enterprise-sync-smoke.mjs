#!/usr/bin/env node

import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";

function parseArgs(argv) {
  const args = {
    baseUrl: process.env.AIPROXY_WEB_BASE_URL || "http://localhost:5173",
    token: process.env.AIPROXY_TEST_TOKEN || "",
    chromePath:
      process.env.AIPROXY_CHROME_PATH ||
      "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    playwrightModuleDir: process.env.PLAYWRIGHT_CORE_MODULE_DIR || "",
    outputDir:
      process.env.AIPROXY_SMOKE_OUTPUT_DIR ||
      path.resolve(process.cwd(), "output/playwright"),
  };

  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    switch (arg) {
      case "--base-url":
        args.baseUrl = argv[++i];
        break;
      case "--token":
        args.token = argv[++i];
        break;
      case "--chrome-path":
        args.chromePath = argv[++i];
        break;
      case "--playwright-module-dir":
        args.playwrightModuleDir = argv[++i];
        break;
      case "--output-dir":
        args.outputDir = argv[++i];
        break;
      case "-h":
      case "--help":
        printHelp();
        process.exit(0);
      default:
        throw new Error(`unknown argument: ${arg}`);
    }
  }

  return args;
}

function printHelp() {
  console.log(`Usage: node scripts/enterprise-sync-smoke.mjs [options]

Options:
  --base-url <url>               Frontend base URL. Default: http://localhost:5173
  --token <token>                Admin/enterprise token used for auth-storage
  --chrome-path <path>           Chrome executable path
  --playwright-module-dir <dir>  Directory containing the playwright-core package
  --output-dir <dir>             Screenshot output directory

Environment variables:
  AIPROXY_WEB_BASE_URL
  AIPROXY_TEST_TOKEN
  AIPROXY_CHROME_PATH
  PLAYWRIGHT_CORE_MODULE_DIR
  AIPROXY_SMOKE_OUTPUT_DIR
`);
}

async function loadPlaywright(moduleDir) {
  if (!moduleDir) {
    throw new Error("PLAYWRIGHT_CORE_MODULE_DIR is required");
  }

  const require = createRequire(import.meta.url);
  return require(path.join(moduleDir, "playwright-core"));
}

function buildAuthStorage(token) {
  return JSON.stringify({
    state: {
      token,
      sessionToken: null,
      isAuthenticated: true,
      enterpriseUser: null,
    },
    version: 0,
  });
}

const SUCCESS_SSE =
  'data: {"type":"success","progress":100,"message":"ok","data":{"success":true,"summary":{"total_models":1,"to_add":0,"to_update":1,"to_delete":0},"details":{"models_added":[],"models_updated":["stub-model"],"models_deleted":[]},"duration_ms":1}}\n\n';

function buildDiagnosticPayload(apiPrefix) {
  const channelsKey = apiPrefix === "ppio" ? "ppio" : "novita";
  return {
    data: {
      last_sync_at: new Date().toISOString(),
      local_models: 1,
      remote_models: 1,
      diff: {
        summary: {
          total_models: 1,
          to_add: 0,
          to_update: 1,
          to_delete: 0,
        },
        changes: {
          add: [],
          update: [
            {
              model_id: "stub-model",
              action: "update",
              changes: ["max_output_tokens"],
              new_config: {
                endpoints: ["anthropic"],
                model_type: "chat",
              },
            },
          ],
          delete: [],
        },
        channels: {
          [channelsKey]: {
            exists: true,
            id: 1,
          },
        },
      },
      channels: {
        [channelsKey]: {
          exists: true,
          id: 1,
        },
      },
    },
    success: true,
  };
}

async function runCase(context, options, name, routePath, apiPrefix, results, consoleErrors, pageErrors) {
  const page = await context.newPage();
  page.on("console", (msg) => {
    if (msg.type() === "error") {
      consoleErrors.push(`${name}:console:${msg.text()}`);
    }
  });
  page.on("pageerror", (err) => {
    pageErrors.push(`${name}:pageerror:${String(err)}`);
  });

  let capturedPayload = null;
  await page.route(`**/api/enterprise/${apiPrefix}/sync/diagnostic`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(buildDiagnosticPayload(apiPrefix)),
    });
  });

  await page.route(`**/api/enterprise/${apiPrefix}/sync/execute`, async (route) => {
    capturedPayload = route.request().postDataJSON();
    await route.fulfill({
      status: 200,
      headers: {
        "Content-Type": "text/event-stream",
        "Cache-Control": "no-cache",
      },
      body: SUCCESS_SSE,
    });
  });

  await page.goto(`${options.baseUrl}${routePath}`, { waitUntil: "domcontentloaded" });
  await page.waitForLoadState("networkidle");

  const switchLocator = page.locator("#anthropic-pure-passthrough");
  await switchLocator.waitFor({ state: "visible", timeout: 15000 });
  const initial = await switchLocator.getAttribute("aria-checked");
  if (initial !== "true") {
    throw new Error(`${name}: expected default switch on, got ${initial}`);
  }

  await page.getByRole("button", { name: /刷新诊断|Refresh Diagnostic/ }).click();

  const execute = page.getByRole("button", { name: /执行同步|Execute Sync/ });
  await execute.waitFor({ state: "visible", timeout: 15000 });
  const disabledBefore = await waitForEnabled(page, execute, 15000);
  if (disabledBefore) {
    const buttons = await page
      .locator("button")
      .evaluateAll((nodes) =>
        nodes.map((n) => ({
          text: n.textContent?.trim(),
          disabled: n.disabled,
          ariaDisabled: n.getAttribute("aria-disabled"),
        })),
      );
    const bodySnippet = (await page.locator("body").textContent())?.slice(0, 2000) || "";
    throw new Error(
      `${name}: execute button disabled after diagnostic\nbuttons=${JSON.stringify(buttons)}\nbody=${bodySnippet}`,
    );
  }

  await switchLocator.click();
  await page.waitForTimeout(300);
  const toggled = await switchLocator.getAttribute("aria-checked");
  if (toggled !== "false") {
    throw new Error(`${name}: expected switch off after click, got ${toggled}`);
  }

  await execute.click();
  await page.waitForTimeout(1800);

  if (!capturedPayload) {
    throw new Error(`${name}: no execute payload captured`);
  }

  if (capturedPayload.anthropic_pure_passthrough !== false) {
    throw new Error(
      `${name}: anthropic_pure_passthrough=${capturedPayload.anthropic_pure_passthrough}`,
    );
  }

  if (capturedPayload.changes_confirmed !== true) {
    throw new Error(`${name}: changes_confirmed=${capturedPayload.changes_confirmed}`);
  }

  const screenshot = path.join(options.outputDir, `${name}.png`);
  await page.screenshot({ path: screenshot, fullPage: true });

  results.push({
    page: name,
    default_checked: initial,
    execute_enabled: !disabledBefore,
    after_toggle: toggled,
    payload: capturedPayload,
    screenshot,
  });

  await page.close();
}

async function waitForEnabled(page, locator, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (!(await locator.isDisabled())) {
      return false;
    }
    await page.waitForTimeout(250);
  }
  return locator.isDisabled();
}

async function main() {
  const options = parseArgs(process.argv.slice(2));
  if (!options.token) {
    throw new Error("missing token: use --token or set AIPROXY_TEST_TOKEN");
  }

  if (!fs.existsSync(options.chromePath)) {
    throw new Error(`chrome not found: ${options.chromePath}`);
  }

  fs.mkdirSync(options.outputDir, { recursive: true });

  const { chromium } = await loadPlaywright(options.playwrightModuleDir);
  const browser = await chromium.launch({
    headless: true,
    executablePath: options.chromePath,
  });

  const context = await browser.newContext();
  await context.addInitScript((storage) => {
    window.localStorage.setItem("auth-storage", storage);
  }, buildAuthStorage(options.token));

  const results = [];
  const consoleErrors = [];
  const pageErrors = [];

  try {
    await runCase(
      context,
      options,
      "ppio-sync-smoke",
      "/enterprise/ppio-sync",
      "ppio",
      results,
      consoleErrors,
      pageErrors,
    );
    await runCase(
      context,
      options,
      "novita-sync-smoke",
      "/enterprise/novita-sync",
      "novita",
      results,
      consoleErrors,
      pageErrors,
    );
  } finally {
    await browser.close();
  }

  const output = { results, consoleErrors, pageErrors };
  console.log(JSON.stringify(output, null, 2));

  if (consoleErrors.length > 0 || pageErrors.length > 0) {
    process.exitCode = 1;
  }
}

main().catch((err) => {
  console.error(err instanceof Error ? err.message : String(err));
  process.exit(1);
});
