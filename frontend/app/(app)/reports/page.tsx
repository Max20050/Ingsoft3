"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import {
  REPORT_FORMATS,
  createReport,
  deleteReport,
  downloadReport,
  listDataSources,
  listReportTemplates,
  listReports,
  type DataSource,
  type Report,
  type ReportFormat,
  type ReportInput,
  type ReportTemplate,
} from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { ReportBuilder } from "@/app/ui/report-builder";

export default function ReportsPage() {
  const router = useRouter();
  const { token, logout } = useAuth();
  const [reports, setReports] = useState<Report[] | null>(null);
  const [sources, setSources] = useState<DataSource[] | null>(null);
  const [templates, setTemplates] = useState<ReportTemplate[]>([]);
  const [error, setError] = useState<string | null>(null);
  // The report being generated, so the button can say so: a download runs the
  // report's query against the user's data source and isn't instant.
  const [running, setRunning] = useState<string | null>(null);

  useEffect(() => {
    if (!token) return;
    Promise.all([listReports(token), listDataSources(token), listReportTemplates(token)])
      .then(([loadedReports, loadedSources, loadedTemplates]) => {
        setReports(loadedReports);
        setSources(loadedSources);
        setTemplates(loadedTemplates);
      })
      .catch(() => {
        logout();
        router.replace("/login");
      });
  }, [token, router, logout]);

  async function handleCreate(input: ReportInput) {
    if (!token) return;
    await createReport(token, input);
    setReports(await listReports(token));
  }

  async function handleDownload(report: Report, format: ReportFormat) {
    if (!token) return;
    setError(null);
    setRunning(`${report.id}:${format}`);
    try {
      const { blob, filename } = await downloadReport(token, report.id, format);
      // The file came back as a response body rather than a URL, so saving it
      // means handing the browser an object URL for one click.
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = filename;
      link.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to generate the report");
    } finally {
      setRunning(null);
    }
  }

  async function handleDelete(id: string) {
    if (!token) return;
    setError(null);
    try {
      await deleteReport(token, id);
      setReports(await listReports(token));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete report");
    }
  }

  if (!token || !reports || !sources) {
    return (
      <div className="flex flex-1 items-center justify-center py-32">
        <p>Loading…</p>
      </div>
    );
  }

  return (
    <div className="flex flex-1 flex-col items-center gap-10 px-6 py-16">
      <div className="flex w-full max-w-2xl items-center justify-between">
        <h1 className="text-2xl font-semibold">Reports</h1>
      </div>

      <div className="flex w-full max-w-2xl flex-col gap-3">
        {reports.length === 0 && (
          <p className="text-sm text-zinc-600 dark:text-zinc-400">No reports created yet.</p>
        )}
        {reports.map((report) => (
          <div
            key={report.id}
            className="flex flex-col gap-2 rounded border border-black/[.1] px-4 py-3 dark:border-white/[.15]"
          >
            <div className="flex items-start justify-between gap-4">
              <div>
                <p className="font-medium">{report.name}</p>
                <p className="text-sm text-zinc-600 dark:text-zinc-400">
                  {report.dataSourceName} ·{" "}
                  {templates.find((template) => template.id === report.templateId)?.name ??
                    report.templateId}{" "}
                  · {report.prompt}
                </p>
              </div>
              <button
                onClick={() => handleDelete(report.id)}
                className="text-sm text-red-600 hover:underline"
              >
                Delete
              </button>
            </div>
            <pre className="overflow-x-auto rounded bg-black/[.05] px-3 py-2 font-mono text-xs dark:bg-white/[.08]">
              {report.query}
            </pre>
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm text-zinc-600 dark:text-zinc-400">Download</span>
              {report.formats.map((format) => (
                <button
                  key={format}
                  onClick={() => handleDownload(report, format)}
                  disabled={running !== null}
                  className="rounded-full border border-black/[.08] px-3 py-1 text-sm transition-colors hover:bg-black/[.04] disabled:opacity-50 dark:border-white/[.145] dark:hover:bg-[#1a1a1a]"
                >
                  {running === `${report.id}:${format}`
                    ? "Generating…"
                    : (REPORT_FORMATS.find((option) => option.value === format)?.label ?? format)}
                </button>
              ))}
            </div>
          </div>
        ))}
      </div>

      {error && <p className="text-sm text-red-600">{error}</p>}

      <div className="flex w-full max-w-2xl flex-col gap-4 border-t border-black/[.1] pt-8 dark:border-white/[.15]">
        <h2 className="text-lg font-semibold">Create a report</h2>
        {sources.length === 0 ? (
          <p className="text-sm text-zinc-600 dark:text-zinc-400">
            <Link href="/datasources" className="underline">
              Connect a data source
            </Link>{" "}
            before creating a report.
          </p>
        ) : (
          <ReportBuilder token={token} sources={sources} onCreate={handleCreate} />
        )}
      </div>
    </div>
  );
}
