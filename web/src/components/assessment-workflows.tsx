"use client";

import * as React from "react";
import Link from "next/link";
import { toast } from "sonner";
import { useI18n } from "@/i18n";
import type { MessageKey } from "@/i18n/catalog";
import { api } from "@/lib/api";
import { assessmentRequestIdentity, acceptAssessmentReceipt } from "@/lib/assessment-request";
import type { AssessmentBlueprint, AssessmentWorkflowCatalog, AssessmentWorkflowRun, ChatAttachment, LLMProfile } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";

const teams = ["all", "red", "purple", "blue"] as const;
type TeamFilter = typeof teams[number];

export default function AssessmentWorkflows() {
  const { t } = useI18n();
  const [catalog, setCatalog] = React.useState<AssessmentWorkflowCatalog | null>(null);
  const [profiles, setProfiles] = React.useState<LLMProfile[]>([]);
  const [runs, setRuns] = React.useState<AssessmentWorkflowRun[]>([]);
  const [selectedID, setSelectedID] = React.useState("full-web");
  const [team, setTeam] = React.useState<TeamFilter>("all");
  const [search, setSearch] = React.useState("");
  const [targets, setTargets] = React.useState("");
  const [context, setContext] = React.useState("");
  const [sourceTasks, setSourceTasks] = React.useState("");
  const [profileID, setProfileID] = React.useState("");
  const [timeoutMinutes, setTimeoutMinutes] = React.useState(30);
  const [attachments, setAttachments] = React.useState<ChatAttachment[]>([]);
  const [loading, setLoading] = React.useState(true);
  const [loadError, setLoadError] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [uploading, setUploading] = React.useState(false);
  const [progressError, setProgressError] = React.useState(false);
  const [focusedRunID, setFocusedRunID] = React.useState<string | null>(null);
  const config = React.useRef<HTMLDivElement>(null);
  const fileInput = React.useRef<HTMLInputElement>(null);
  const uploadID = React.useRef("");
  // Catalog names and procedures are English-authored source content, like
  // imported skill metadata. Interface controls use the six-language catalog.
  const title = (row: AssessmentBlueprint) => row.title;
  const summary = (row: AssessmentBlueprint) => row.summary;

  const load = React.useCallback(async () => {
    setLoading(true); setLoadError("");
    try {
      const [next, models, history] = await Promise.all([api.assessmentWorkflows(), api.llmProfiles(), api.assessmentWorkflowRuns()]);
      setCatalog(next); setProfiles(models); setRuns(history); setProgressError(false);
      if (history[0] && next.workflows.some((item) => item.id === history[0].workflow.key)) {
        setSelectedID((current) => current === "full-web" ? history[0].workflow.key : current);
      }
    } catch (error) { setLoadError((error as Error).message); }
    finally { setLoading(false); }
  }, []);
  React.useEffect(() => { void load(); }, [load]);
  React.useEffect(() => {
    if (!catalog) return;
    let current = true;
    const timer = setInterval(() => {
      void api.assessmentWorkflowRuns().then((rows) => { if (current) { setRuns(rows); setProgressError(false); } }).catch(() => { if (current) setProgressError(true); });
    }, 5000);
    return () => { current = false; clearInterval(timer); };
  }, [catalog]);

  const selected = catalog?.workflows.find((item) => item.id === selectedID) ?? catalog?.workflows[0];
  const shown = catalog?.workflows.filter((item) => (team === "all" || item.team === team) &&
    `${title(item)} ${summary(item)} ${item.skills.join(" ")}`.toLowerCase().includes(search.toLowerCase())) ?? [];
  const configured = selected && selected.missing_skills.length === 0 && selected.unbound_skills.length === 0;
  const activeRun = runs.find((run) => run.task.id === focusedRunID) ?? runs[0];

  function choose(item: AssessmentBlueprint) {
    setSelectedID(item.id);
    if (window.matchMedia("(max-width: 1023px)").matches) requestAnimationFrame(() => config.current?.scrollIntoView({ block: "start" }));
  }
  async function upload(files: File[]) {
    if (!files.length) return;
    if (files.some((file) => file.size > 32 * 1024 * 1024) || attachments.length + files.length > 16) { toast.error(t("operations.uploadLimit")); return; }
    setUploading(true);
    try {
      uploadID.current ||= crypto.randomUUID();
      const result = await api.chatUpload("staging", uploadID.current, files);
      setAttachments((items) => [...items, ...result.attachments]);
    } catch (error) { toast.error((error as Error).message); }
    finally { setUploading(false); if (fileInput.current) fileInput.current.value = ""; }
  }
  async function start() {
    if (!selected || !configured || busy || uploading) return;
    const refs = targets.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
    const sourceIDs = [...new Set(sourceTasks.split(/[\s,]+/).map((value) => value.replace(/^#/, "")).filter(Boolean))];
    if (sourceIDs.some((id) => !/^[1-9]\d*$/.test(id))) { toast.error(t("operations.invalidSources")); return; }
    if (profileID && !/^[1-9]\d*$/.test(profileID)) { toast.error(t("operations.invalidModel")); return; }
    const evidence = attachments.map((item) => ({ name: item.name, server_path: item.abs ?? item.path }));
    const withEvidence = context + (evidence.length ? `\n\nUploaded evidence references:\n${JSON.stringify(evidence, null, 2)}` : "");
    const scope = refs.length ? refs : evidence.map((item) => item.server_path);
    if (!scope.length || new TextEncoder().encode(withEvidence).length > 16000) { toast.error(t("operations.invalidInputs")); return; }
    setBusy(true);
    try {
      const input = {
        targets: scope, context: withEvidence, llm_profile_ids: profileID ? [profileID] : [], source_task_ids: sourceIDs, timeout_seconds: timeoutMinutes * 60,
      };
      const receipt = await assessmentRequestIdentity(selected.id, input);
      const result = await api.runAssessmentWorkflow(selected.id, { ...input, client_request_id: receipt.id });
      acceptAssessmentReceipt(receipt);
      setFocusedRunID(result.task.id);
      toast.success(t("operations.started"));
      try { setRuns(await api.assessmentWorkflowRuns()); setProgressError(false); }
      catch { setProgressError(true); }
      requestAnimationFrame(() => document.getElementById("assessment-progress")?.scrollIntoView({ block: "start" }));
    } catch (error) { toast.error((error as Error).message); }
    finally { setBusy(false); }
  }

  return <div className="flex min-w-0 flex-col gap-4">
    <div className="flex flex-wrap items-start justify-between gap-3"><div>
      <h1 className="text-xl font-semibold">{t("workflows.title")}</h1>
      <p className="max-w-3xl text-sm text-muted-foreground">{t("operations.introduction")}</p>
    </div>{catalog && <Badge variant="outline">{t("operations.catalogCount", { count: catalog.workflows.length, skills: catalog.bundled_skills.length })}</Badge>}</div>
    {loading && <p role="status">{t("workflows.loading")}</p>}
    {loadError && <div role="alert" className="flex flex-wrap items-center gap-3 rounded-md border border-destructive p-3 text-sm"><span>{t("operations.loadFailed")}: {loadError}</span><Button variant="outline" onClick={() => void load()}>{t("workflows.retry")}</Button></div>}
    {progressError && <p role="status" className="rounded-md border p-3 text-sm text-muted-foreground">{t("operations.progressUnavailable")}</p>}
    {catalog && <div className="grid min-w-0 gap-4 lg:grid-cols-2">
      <Card><CardHeader className="gap-3"><CardTitle>{t("operations.chooseWorkflow")}</CardTitle>
        <div role="group" aria-label={t("operations.teamFilter")} className="flex flex-wrap gap-2">
          {teams.map((value) => <Button key={value} size="sm" variant={team === value ? "default" : "outline"} aria-pressed={team === value} onClick={() => setTeam(value)}>{t(`operations.team_${value}`)}</Button>)}
        </div>
        <Input aria-label={t("operations.search")} placeholder={t("operations.search")} value={search} onChange={(event) => setSearch(event.target.value)} />
      </CardHeader><CardContent className="grid max-h-[36rem] gap-2 overflow-y-auto sm:grid-cols-2">
        {shown.length === 0 && <p className="text-sm text-muted-foreground">{t("operations.noMatches")}</p>}
        {shown.map((item) => <button type="button" key={item.id} data-assessment-workflow={item.id} aria-pressed={selected?.id === item.id} onClick={() => choose(item)}
          className={`flex flex-col gap-2 rounded-md border p-3 text-left text-sm hover:bg-muted ${selected?.id === item.id ? "border-primary bg-muted" : ""}`}>
          <Badge variant="outline" className="w-fit">{t(`operations.team_${item.team}`)}</Badge>
          <span className="font-medium">{title(item)}</span><span className="text-xs text-muted-foreground">{summary(item)}</span>
        </button>)}
      </CardContent></Card>
      {selected && <Card ref={config}><CardHeader><CardTitle>{title(selected)}</CardTitle><p className="text-sm text-muted-foreground">{summary(selected)}</p></CardHeader><CardContent className="grid gap-4">
        <div className="grid gap-1"><Label htmlFor="assessment-targets">{selected.network ? t("operations.targets") : t("operations.evidence")}</Label>
          <Textarea id="assessment-targets" rows={3} maxLength={8000} value={targets} onChange={(event) => setTargets(event.target.value)} placeholder={selected.network ? t("operations.targetsHint") : t("operations.evidenceHint")} /></div>
        <div className="grid gap-1"><Label htmlFor="assessment-model">{t("operations.model")}</Label><select id="assessment-model" className="h-9 min-w-0 rounded-md border bg-background px-2 text-sm" value={profileID} onChange={(event) => setProfileID(event.target.value)}>
          <option value="">{t("operations.defaultModel")}</option>{profiles.map((profile) => <option key={profile.id} value={profile.id}>{profile.name} · {profile.model}</option>)}
        </select><p className="text-xs text-muted-foreground">{t("operations.modelHelp")}</p></div>
        <div className="grid gap-1"><Label htmlFor="assessment-context">{t("operations.context")}</Label><Textarea id="assessment-context" rows={3} maxLength={4000} value={context} onChange={(event) => setContext(event.target.value)} placeholder={t("operations.contextHint")} /></div>
        <input ref={fileInput} type="file" multiple className="hidden" onChange={(event) => void upload(Array.from(event.target.files ?? []))} />
        <div className="flex flex-wrap items-center gap-2"><Button variant="outline" disabled={uploading || busy} onClick={() => fileInput.current?.click()}>{uploading ? t("operations.uploading") : t("operations.attachEvidence")}</Button>
          {attachments.map((item, index) => <Button key={`${item.path}-${index}`} variant="ghost" size="sm" aria-label={t("operations.removeEvidence", { name: item.name })} onClick={() => setAttachments((items) => items.filter((_, i) => i !== index))}>{item.name} ×</Button>)}
        </div>
        {selected.team === "purple" && <p className="rounded-md border p-3 text-sm text-muted-foreground">{t("operations.telemetryHelp")}</p>}
        <details className="rounded-md border p-3"><summary className="cursor-pointer text-sm font-medium">{t("operations.advanced")}</summary><div className="mt-3 grid gap-3">
          <div className="grid gap-1"><Label htmlFor="assessment-sources">{t("operations.sourceTasks")}</Label><Input id="assessment-sources" value={sourceTasks} onChange={(event) => setSourceTasks(event.target.value)} placeholder="42, 57" /></div>
          <div className="grid gap-1"><Label htmlFor="assessment-timeout">{t("operations.timeLimit")}</Label><Input id="assessment-timeout" type="number" min={1} max={1440} value={timeoutMinutes} onChange={(event) => setTimeoutMinutes(Number(event.target.value) || 0)} /></div>
          <p className="break-words text-xs text-muted-foreground">{t("operations.sourceProjects")}: {selected.sources.join(", ")}</p>
        </div></details>
        <div className="grid gap-2"><p className="text-sm font-medium">{t("operations.connectedPlaybooks")}</p><div className="flex flex-wrap gap-1">{selected.skills.map((skill) => <Badge key={skill} variant="outline">{skill}</Badge>)}</div>
          {!configured && <p role="status" className="text-sm text-destructive">{t("operations.needsSetup")}: {[...new Set([...selected.missing_skills, ...selected.unbound_skills])].join(", ")} <Link className="underline" href="/system/agents">{t("operations.configureAgents")}</Link></p>}
        </div>
        {selected.prerequisites.length > 0 && <div className="grid gap-1 text-sm"><p className="font-medium">{t("operations.prerequisites")}</p><ul className="list-disc space-y-1 pl-5 text-muted-foreground">{selected.prerequisites.map((text) => <li key={text}>{text}</li>)}</ul></div>}
        {selected.execution_notes.length > 0 && <div role="status" className="grid gap-1 rounded-md border border-amber-500/40 bg-amber-500/5 p-3 text-sm"><p className="font-medium">{t("operations.executionNotes")}</p><ul className="list-disc space-y-1 pl-5">{selected.execution_notes.map((text) => <li key={text}>{text}</li>)}</ul></div>}
        <div className="grid gap-2"><p className="text-sm font-medium">{t("operations.procedure")}</p><ol className="list-decimal space-y-2 pl-5 text-sm text-muted-foreground">{selected.procedure.map((text, index) => <li key={index}>{text}</li>)}</ol></div>
        <ol className="grid gap-2" aria-label={t("operations.executionPlan")}>{selected.phases.map((phase, index) => <li key={phase.id} className="flex items-center gap-2 rounded-md border px-3 py-2 text-sm"><Badge variant="outline">{index + 1}</Badge>{t(`operations.phase_${phase.id}` as MessageKey)}</li>)}</ol>
        <Button disabled={loading || busy || uploading || !configured || (!targets.trim() && (selected.network || !attachments.length)) || timeoutMinutes < 1 || timeoutMinutes > 1440} onClick={() => void start()}>{busy ? t("operations.starting") : t("operations.start")}</Button>
      </CardContent></Card>}
    </div>}
    {activeRun && <Card id="assessment-progress"><CardHeader><div className="flex flex-wrap items-center justify-between gap-2"><CardTitle>{t("operations.progress")}</CardTitle><Button asChild variant="outline"><Link href={`/function/tasks/detail?id=${encodeURIComponent(activeRun.task.id)}`}>{t("operations.openTask")}</Link></Button></div>
      <p className="text-sm text-muted-foreground">{activeRun.workflow.title} · #{activeRun.task.id} · {activeRun.task.status}</p>
    </CardHeader><CardContent className="grid gap-3">
      <ol className="grid gap-2 md:grid-cols-5">{activeRun.phases.map((phase, index) => <li key={phase.id} data-assessment-phase={phase.id} data-phase-state={phase.state} className={`min-w-0 rounded-md border p-3 text-sm ${phase.state === "met" ? "border-primary bg-primary/10" : ""}`}>
        <p className="font-medium">{index + 1}. {t(`operations.phase_${phase.id}` as MessageKey)}</p><p className="mt-1 text-xs text-muted-foreground">{t(phase.state === "met" ? "operations.phaseComplete" : phase.state === "missing" ? "operations.phaseMissing" : "operations.phaseOpen")}</p>
      </li>)}</ol>
      <p className="text-xs text-muted-foreground">{t("operations.progressHelp")}</p>
    </CardContent></Card>}
    {runs.length > 0 && <Card><CardHeader><CardTitle>{t("operations.recentRuns")}</CardTitle></CardHeader><CardContent className="grid gap-2">
      {runs.map((run) => <button key={run.task.id} type="button" className="flex flex-wrap items-center justify-between gap-2 rounded-md border p-3 text-left text-sm hover:bg-muted" onClick={() => setFocusedRunID(run.task.id)}>
        <span className="font-medium">{run.workflow.title} · #{run.task.id}</span><span className="text-muted-foreground">{run.phases.filter((phase) => phase.state === "met").length}/{run.phases.length} · {run.task.status}</span>
      </button>)}
    </CardContent></Card>}
  </div>;
}
