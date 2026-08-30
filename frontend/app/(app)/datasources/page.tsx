"use client";

import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import {
  createDataSource,
  createGoogleSheetsDataSource,
  createRestApiDataSource,
  deleteDataSource,
  listDataSources,
  testDataSource,
  testRestApiDataSource,
  type DataSource,
  type DataSourceInput,
  type GoogleSheetsDataSourceInput,
  type RestApiDataSourceInput,
} from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { DataSourceForm } from "@/app/ui/data-source-form";
import { GoogleSheetsConnectButton } from "@/app/ui/google-sheets-connect-button";
import { GoogleSheetsPicker } from "@/app/ui/google-sheets-picker";
import { DataSourceSchemaView } from "@/app/ui/data-source-schema";
import { RestApiDataSourceForm } from "@/app/ui/rest-api-datasource-form";

function sourceSubtitle(source: DataSource): string {
  if (source.type === "google_sheets") {
    return `Google Sheets · ${source.spreadsheetName ?? source.spreadsheetId}`;
  }
  if (source.type === "rest_api") {
    return `REST API · ${source.method ?? "GET"} ${source.url}`;
  }
  return `${source.type} · ${source.host}:${source.port}/${source.dbName}`;
}

function DataSourcesContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { token, logout } = useAuth();
  const [sources, setSources] = useState<DataSource[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const sheetsConnectionId = searchParams.get("sheetsConnection");

  useEffect(() => {
    if (!token) return;
    listDataSources(token)
      .then(setSources)
      .catch(() => {
        logout();
        router.replace("/login");
      });
  }, [token, router, logout]);

  async function handleTest(input: DataSourceInput) {
    if (!token) return;
    await testDataSource(token, input);
  }

  async function handleCreate(input: DataSourceInput) {
    if (!token) return;
    await createDataSource(token, input);
    setSources(await listDataSources(token));
  }

  async function handleCreateSheetsSource(input: GoogleSheetsDataSourceInput) {
    if (!token) return;
    await createGoogleSheetsDataSource(token, input);
    setSources(await listDataSources(token));
    router.replace("/datasources");
  }

  async function handleTestRest(input: RestApiDataSourceInput) {
    if (!token) return;
    await testRestApiDataSource(token, input);
  }

  async function handleCreateRest(input: RestApiDataSourceInput) {
    if (!token) return;
    await createRestApiDataSource(token, input);
    setSources(await listDataSources(token));
  }

  async function handleDelete(id: string) {
    if (!token) return;
    setError(null);
    try {
      await deleteDataSource(token, id);
      if (selectedId === id) setSelectedId(null);
      setSources(await listDataSources(token));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete data source");
    }
  }

  if (!token || !sources) {
    return (
      <div className="flex flex-1 items-center justify-center py-32">
        <p>Loading…</p>
      </div>
    );
  }

  return (
    <div className="flex flex-1 flex-col items-center gap-10 px-6 py-16">
      <div className="flex w-full max-w-md items-center justify-between">
        <h1 className="text-2xl font-semibold">Connectors</h1>
      </div>

      <div className="flex w-full max-w-md flex-col gap-3">
        {sources.length === 0 && (
          <p className="text-sm text-zinc-600 dark:text-zinc-400">No data sources connected yet.</p>
        )}
        {sources.map((source) => (
          <div
            key={source.id}
            className="flex flex-col gap-3 rounded border border-black/[.1] px-4 py-3 dark:border-white/[.15]"
          >
            <div className="flex items-start justify-between gap-4">
              <div className="min-w-0 flex-1">
                <p className="font-medium">{source.name}</p>
                <p className="break-words text-sm text-zinc-600 dark:text-zinc-400">{sourceSubtitle(source)}</p>
              </div>
              <div className="flex shrink-0 items-center gap-4">
                <button
                  onClick={() => setSelectedId(selectedId === source.id ? null : source.id)}
                  className="text-sm underline"
                >
                  {selectedId === source.id ? "Hide schema" : "View schema"}
                </button>
                <button onClick={() => handleDelete(source.id)} className="text-sm text-red-600 hover:underline">
                  Delete
                </button>
              </div>
            </div>
            {selectedId === source.id && (
              <div className="flex flex-col gap-4 border-t border-black/[.1] pt-3 dark:border-white/[.15]">
                <DataSourceSchemaView
                  key={source.id}
                  token={token}
                  dataSourceId={source.id}
                  type={source.type}
                />
              </div>
            )}
          </div>
        ))}
      </div>

      {error && <p className="text-sm text-red-600">{error}</p>}

      <div className="flex w-full max-w-md flex-col gap-4 border-t border-black/[.1] pt-8 dark:border-white/[.15]">
        <h2 className="text-lg font-semibold">Connect Google Sheets</h2>
        {sheetsConnectionId ? (
          <GoogleSheetsPicker
            token={token}
            connectionId={sheetsConnectionId}
            onCreate={handleCreateSheetsSource}
          />
        ) : (
          <GoogleSheetsConnectButton token={token} />
        )}
      </div>

      <div className="flex w-full max-w-md flex-col gap-4 border-t border-black/[.1] pt-8 dark:border-white/[.15]">
        <h2 className="text-lg font-semibold">Add a SQL data source</h2>
        <DataSourceForm onTest={handleTest} onCreate={handleCreate} />
      </div>

      <div className="flex w-full max-w-md flex-col gap-4 border-t border-black/[.1] pt-8 dark:border-white/[.15]">
        <h2 className="text-lg font-semibold">Add a REST API data source</h2>
        <RestApiDataSourceForm onTest={handleTestRest} onCreate={handleCreateRest} />
      </div>
    </div>
  );
}

export default function DataSourcesPage() {
  return (
    <Suspense
      fallback={
        <div className="flex flex-1 items-center justify-center py-32">
          <p>Loading…</p>
        </div>
      }
    >
      <DataSourcesContent />
    </Suspense>
  );
}
