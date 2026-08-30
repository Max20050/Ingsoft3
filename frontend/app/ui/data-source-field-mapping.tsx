"use client";

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { getDataSourceSchema, getFieldMapping, saveFieldMapping, type SystemField } from "@/lib/api";

type Point = { x: number; y: number };

// DataSourceFieldMappingPanel lets the user connect a REST data source's
// detected api fields (left column) to DocuWave's predefined system fields
// (right column) by dragging from one to the other. Connections are drawn as
// SVG lines over the two columns and persisted to the backend on every
// change, so a reload recreates them from the saved mapping.
//
// Line coordinates are DOM measurements (element positions), not derived
// data, so they're written straight onto the SVG elements via refs instead
// of round-tripping through React state — that keeps position updates
// (on drag, resize, and mapping changes) out of the render path entirely.
export function DataSourceFieldMappingPanel({
  token,
  dataSourceId,
  onMappingChange,
}: {
  token: string;
  dataSourceId: string;
  // onMappingChange fires with the freshly-saved mapping whenever a
  // connection is made or removed, so a caller that needs its own copy of
  // the mapping (the report builder's field picker, keyed off it) can stay
  // in sync without re-fetching on a timer.
  onMappingChange?: (mapping: Record<string, string>) => void;
}) {
  const [apiFields, setApiFields] = useState<string[] | null>(null);
  const [systemFields, setSystemFields] = useState<SystemField[]>([]);
  const [mapping, setMapping] = useState<Record<string, string> | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [dragging, setDragging] = useState(false);

  const containerRef = useRef<HTMLDivElement>(null);
  const leftRefs = useRef<Record<string, HTMLDivElement | null>>({});
  const rightRefs = useRef<Record<string, HTMLDivElement | null>>({});
  const lineRefs = useRef<Record<string, SVGLineElement | null>>({});
  const dragLineRef = useRef<SVGLineElement | null>(null);
  const dragFromRef = useRef<string | null>(null);
  const dragOriginRef = useRef<Point | null>(null);

  useEffect(() => {
    let active = true;
    Promise.all([getDataSourceSchema(token, dataSourceId), getFieldMapping(token, dataSourceId)])
      .then(([schema, fieldMapping]) => {
        if (!active) return;
        setApiFields(schema.fields ?? []);
        setSystemFields(fieldMapping.systemFields);
        setMapping(fieldMapping.mapping);
      })
      .catch((err) => {
        if (active) setError(err instanceof Error ? err.message : "Failed to load field mapping");
      });
    return () => {
      active = false;
    };
  }, [token, dataSourceId]);

  const centerOf = useCallback((el: HTMLDivElement | null, side: "left" | "right"): Point | null => {
    const container = containerRef.current;
    if (!el || !container) return null;
    const rect = el.getBoundingClientRect();
    const containerRect = container.getBoundingClientRect();
    return {
      x: (side === "left" ? rect.right : rect.left) - containerRect.left,
      y: rect.top + rect.height / 2 - containerRect.top,
    };
  }, []);

  const repositionLines = useCallback(() => {
    if (!mapping) return;
    for (const [apiField, systemFieldKey] of Object.entries(mapping)) {
      const lineEl = lineRefs.current[apiField];
      const from = centerOf(leftRefs.current[apiField], "left");
      const to = centerOf(rightRefs.current[systemFieldKey], "right");
      if (!lineEl || !from || !to) continue;
      lineEl.setAttribute("x1", String(from.x));
      lineEl.setAttribute("y1", String(from.y));
      lineEl.setAttribute("x2", String(to.x));
      lineEl.setAttribute("y2", String(to.y));
    }
  }, [mapping, centerOf]);

  // Repositions the fixed connection lines whenever the mapping changes or
  // the rows they point at might have moved, reading refs only here (after
  // paint), never inline during render.
  useLayoutEffect(() => {
    repositionLines();
  }, [repositionLines]);

  useEffect(() => {
    window.addEventListener("resize", repositionLines);
    return () => window.removeEventListener("resize", repositionLines);
  }, [repositionLines]);

  useEffect(() => {
    if (!dragging) return;

    const handleMove = (e: MouseEvent) => {
      const container = containerRef.current;
      const lineEl = dragLineRef.current;
      const origin = dragOriginRef.current;
      if (!container || !lineEl || !origin) return;
      const containerRect = container.getBoundingClientRect();
      lineEl.setAttribute("x1", String(origin.x));
      lineEl.setAttribute("y1", String(origin.y));
      lineEl.setAttribute("x2", String(e.clientX - containerRect.left));
      lineEl.setAttribute("y2", String(e.clientY - containerRect.top));
    };
    const handleUp = () => {
      dragFromRef.current = null;
      dragOriginRef.current = null;
      setDragging(false);
    };
    window.addEventListener("mousemove", handleMove);
    window.addEventListener("mouseup", handleUp);
    return () => {
      window.removeEventListener("mousemove", handleMove);
      window.removeEventListener("mouseup", handleUp);
    };
  }, [dragging]);

  function persist(next: Record<string, string>) {
    setMapping(next);
    setSaving(true);
    setError(null);
    saveFieldMapping(token, dataSourceId, next)
      .then((result) => {
        setMapping(result.mapping);
        onMappingChange?.(result.mapping);
      })
      .catch((err) => setError(err instanceof Error ? err.message : "Failed to save field mapping"))
      .finally(() => setSaving(false));
  }

  function connect(apiField: string, systemFieldKey: string) {
    if (!mapping) return;
    // A field can only ever point to one target, so a new connection from an
    // already-mapped source replaces the old one.
    persist({ ...mapping, [apiField]: systemFieldKey });
  }

  function disconnect(apiField: string) {
    if (!mapping) return;
    const next = { ...mapping };
    delete next[apiField];
    persist(next);
  }

  function handleDragStart(apiField: string) {
    const origin = centerOf(leftRefs.current[apiField], "left");
    if (!origin) return;
    dragFromRef.current = apiField;
    dragOriginRef.current = origin;
    setDragging(true);
  }

  function handleDrop(systemFieldKey: string) {
    if (dragFromRef.current) connect(dragFromRef.current, systemFieldKey);
    dragFromRef.current = null;
    dragOriginRef.current = null;
    setDragging(false);
  }

  if (error && (!mapping || !apiFields)) {
    return <p className="text-sm text-red-600">{error}</p>;
  }

  if (!mapping || !apiFields) {
    return <p className="text-sm text-zinc-600 dark:text-zinc-400">Loading field mapping…</p>;
  }

  if (apiFields.length === 0) {
    return <p className="text-sm text-zinc-600 dark:text-zinc-400">No fields detected yet.</p>;
  }

  return (
    <div className="flex flex-col gap-2">
      <div ref={containerRef} className="relative flex justify-between gap-8">
        <svg className="pointer-events-none absolute inset-0 h-full w-full">
          {Object.keys(mapping).map((apiField) => (
            <line
              key={apiField}
              ref={(el) => {
                lineRefs.current[apiField] = el;
              }}
              stroke="currentColor"
              strokeWidth={1.5}
              className="cursor-pointer text-blue-500 [pointer-events:stroke]"
              onClick={() => disconnect(apiField)}
            />
          ))}
          <line
            ref={dragLineRef}
            stroke="currentColor"
            strokeWidth={1.5}
            strokeDasharray="4 3"
            className="text-blue-400"
            style={{ visibility: dragging ? "visible" : "hidden" }}
          />
        </svg>

        <div className="z-10 flex flex-1 flex-col gap-2">
          {apiFields.map((field) => (
            <div
              key={field}
              ref={(el) => {
                leftRefs.current[field] = el;
              }}
              onMouseDown={() => handleDragStart(field)}
              className="cursor-grab rounded bg-black/[.05] px-2 py-1 text-right font-mono text-xs active:cursor-grabbing dark:bg-white/[.08]"
            >
              {field || "(unnamed)"}
            </div>
          ))}
        </div>

        <div className="z-10 flex flex-1 flex-col gap-2">
          {systemFields.map((field) => (
            <div
              key={field.key}
              ref={(el) => {
                rightRefs.current[field.key] = el;
              }}
              onMouseUp={() => handleDrop(field.key)}
              className="rounded border border-dashed border-black/[.15] px-2 py-1 text-xs dark:border-white/[.2]"
            >
              {field.label}
            </div>
          ))}
        </div>
      </div>

      <p className="text-xs text-zinc-500">
        Arrastrá desde un campo detectado hacia un campo del sistema para conectarlos. Hacé click en una
        línea para eliminar la conexión.
      </p>
      {saving && <p className="text-xs text-zinc-500">Saving…</p>}
      {error && <p className="text-sm text-red-600">{error}</p>}
    </div>
  );
}
