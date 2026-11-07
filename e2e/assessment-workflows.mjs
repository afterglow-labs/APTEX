import { LOCALES, namespaceMessages } from "./locale-resources.mjs";

const text = namespaceMessages("operations");

// Exercises the task-backed catalog against the disposable smoke database.
// The smoke server has no model, so this verifies admission, durable phases,
// scope, and UI recovery; model-driven execution is tested separately.
export async function runAssessmentChecks({ base, token, step, check, openPage, out }) {
  const headers = { Authorization: `Bearer ${token}` };
  for (const locale of LOCALES) {
    await step(`[${locale}] task-backed purple workflow persists through reload`, async () => {
      const page = await openPage(locale, token);
      let taskID = "";
      try {
        await page.goto(`${base}/function/workflows`);
        await page.getByRole("tab", { name: text[locale].runWorkflows, exact: true }).waitFor();
        await page.locator('[data-assessment-workflow="purple-web"]').waitFor();
        check(await page.locator("[data-assessment-workflow]").count() === 29, "catalog lost a workflow");
        await page.locator('[data-assessment-workflow="purple-web"]').click();
        await page.getByText(text[locale].procedure, { exact: true }).waitFor();
        check(await page.locator("#assessment-targets").isVisible(), "target form missing");
        await page.locator("#assessment-targets").fill(`https://fixture-${locale.toLowerCase()}.example.test`);
        await page.locator("#assessment-context").fill("Owned disposable fixture; preserve observed evidence and label missing telemetry.");
        const admission = page.waitForResponse((response) =>
          new URL(response.url()).pathname === "/api/assessment-workflows/purple-web/run" && response.request().method() === "POST");
        await page.getByRole("button", { name: text[locale].start, exact: true }).click();
        const response = await admission;
        check(response.status() === 201, `assessment admission returned ${response.status()}`);
        const receipt = await response.json();
        taskID = receipt.task.id;
        check(/^\d+$/.test(taskID), "assessment did not create a task");
        const requestBody = response.request().postDataJSON();
        const replay = await page.request.post(`${base}/api/assessment-workflows/purple-web/run`, { headers, data: requestBody });
        check([200, 202].includes(replay.status()), `idempotent retry returned ${replay.status()}`);
        check((await replay.json()).task.id === taskID, "retry created a second task");
        const conflict = await page.request.post(`${base}/api/assessment-workflows/purple-web/run`,
          { headers, data: { ...requestBody, context: "changed payload" } });
        check(conflict.status() === 409, "request ID accepted changed inputs");
        await page.locator("#assessment-progress").waitFor();
        check((await page.locator("[data-assessment-phase]").count()) === 5, "persisted phases missing");
        const taskLink = page.getByRole("link", { name: text[locale].openTask, exact: true });
        check((await taskLink.getAttribute("href"))?.includes(`id=${taskID}`), "progress did not link to the real task");
        await page.reload();
        await page.locator("#assessment-progress").waitFor();
        check((await page.locator("[data-assessment-phase]").count()) === 5, "reload lost persisted phase state");
        await page.locator('[data-assessment-workflow="purple-web"][aria-pressed="true"]').waitFor({ timeout: 5000 });
        if (locale === "en") await page.screenshot({ path: `${out}/assessment-purple-web-en.png`, fullPage: true });
      } finally {
        if (taskID) await page.request.delete(`${base}/api/tasks/${taskID}`, { headers, data: {} });
        await page.context().close();
      }
    });
  }

  await step("[mobile] assessment catalog and setup notes are visible", async () => {
    const page = await openPage("en", token);
    try {
      await page.setViewportSize({ width: 390, height: 844 });
      await page.goto(`${base}/function/workflows`);
      await page.locator('[data-assessment-workflow="reverse"]').click();
      await page.getByText(text.en.executionNotes, { exact: true }).waitFor();
      await page.getByText(text.en.procedure, { exact: true }).waitFor();
      await page.screenshot({ path: `${out}/assessment-reverse-mobile.png`, fullPage: true });
    } finally { await page.context().close(); }
  });
}
