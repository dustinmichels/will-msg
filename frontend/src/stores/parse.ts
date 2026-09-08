import { defineStore } from "pinia";
import type { appservice, engine, scanner } from "wailsjs/go/models";
import * as Service from "wailsjs/go/appservice/Service";

export type ParseStatus = "idle" | "scanning" | "parsing" | "ready" | "error";

export interface ParseState {
  sourcePaths: string[];
  sources: scanner.MessageSource[];
  records: engine.Record[];
  skipped: appservice.SkippedSource[];
  requestEpoch: number;
  status: ParseStatus;
  errorMessage: string | null;
}

export const useParseStore = defineStore("parse", {
  state: (): ParseState => ({
    sourcePaths: [],
    sources: [],
    records: [],
    skipped: [],
    requestEpoch: 0,
    status: "idle",
    errorMessage: null,
  }),

  getters: {
    hasSources: (state): boolean => state.sources.length > 0,
    hasRecords: (state): boolean => state.records.length > 0,
    hasSkipped: (state): boolean => state.skipped.length > 0,
    isScanning: (state): boolean => state.status === "scanning",
    isParsing: (state): boolean => state.status === "parsing",
    isProcessing: (state): boolean => state.status === "scanning" || state.status === "parsing",
  },

  actions: {
    async scanSources(paths: string[]): Promise<appservice.ScanResult | null> {
      if (!paths || paths.length === 0) {
        return null;
      }

      const epoch = ++this.requestEpoch;
      this.status = "scanning";
      this.errorMessage = null;

      try {
        const res = await Service.ScanSources(paths);
        if (epoch !== this.requestEpoch) {
          // Stale request superseded by a newer action
          return null;
        }

        this.sourcePaths = res.source_paths || [];
        this.sources = res.sources || [];
        this.records = [];
        this.skipped = [];
        this.status = "idle";
        return res;
      } catch (err: unknown) {
        if (epoch !== this.requestEpoch) {
          return null;
        }
        this.status = "error";
        this.errorMessage = err instanceof Error ? err.message : String(err);
        throw err;
      }
    },

    async selectFiles(): Promise<appservice.ScanResult | null> {
      try {
        const paths = await Service.SelectFiles();
        if (!paths || paths.length === 0) {
          return null;
        }
        return await this.scanSources(paths);
      } catch (err: unknown) {
        this.status = "error";
        this.errorMessage = err instanceof Error ? err.message : String(err);
        throw err;
      }
    },

    async selectFolder(): Promise<appservice.ScanResult | null> {
      try {
        const folder = await Service.SelectFolder();
        if (!folder) {
          return null;
        }
        return await this.scanSources([folder]);
      } catch (err: unknown) {
        this.status = "error";
        this.errorMessage = err instanceof Error ? err.message : String(err);
        throw err;
      }
    },

    async parse(): Promise<appservice.ParseResult | null> {
      if (this.sources.length === 0) {
        return null;
      }

      const epoch = ++this.requestEpoch;
      this.status = "parsing";
      this.errorMessage = null;

      try {
        const res = await Service.Parse(this.sources);
        if (epoch !== this.requestEpoch) {
          // Stale parse superseded by a newer request or reset
          return null;
        }

        if (res.superseded) {
          // Parse was superseded on the backend
          return res;
        }

        this.records = res.records || [];
        this.skipped = res.skipped || [];
        this.status = "ready";
        return res;
      } catch (err: unknown) {
        if (epoch !== this.requestEpoch) {
          return null;
        }
        this.status = "error";
        this.errorMessage = err instanceof Error ? err.message : String(err);
        throw err;
      }
    },

    async reset(): Promise<void> {
      this.requestEpoch++;
      this.sourcePaths = [];
      this.sources = [];
      this.records = [];
      this.skipped = [];
      this.status = "idle";
      this.errorMessage = null;

      try {
        await Service.ClearParse();
      } catch (err: unknown) {
        console.error("Error clearing parse state on backend:", err);
      }
    },

    async saveToDownloads(): Promise<appservice.SavedFile> {
      return await Service.SaveToDownloads();
    },

    async saveAs(): Promise<appservice.SavedFile> {
      return await Service.SaveAs();
    },

    async revealFile(path: string): Promise<void> {
      await Service.RevealFile(path);
    },
  },
});
