import { describe, expect, it, beforeEach } from "bun:test";
import { setActivePinia, createPinia } from "pinia";
import { useParseStore } from "./parse";
import { appservice, engine, scanner } from "wailsjs/go/models";

describe("Parse Store", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    // Set up mock window.go bindings
    const globalObj = globalThis as unknown as {
      window: {
        go: {
          appservice: {
            Service: Record<string, unknown>;
          };
        };
      };
    };
    globalObj.window = {
      go: {
        appservice: {
          Service: {},
        },
      },
    };
  });

  it("initializes with empty state and idle status", () => {
    const store = useParseStore();
    expect(store.sourcePaths).toEqual([]);
    expect(store.sources).toEqual([]);
    expect(store.records).toEqual([]);
    expect(store.skipped).toEqual([]);
    expect(store.requestEpoch).toBe(0);
    expect(store.status).toBe("idle");
    expect(store.hasSources).toBe(false);
    expect(store.hasRecords).toBe(false);
  });

  it("scans sources successfully and updates state", async () => {
    const mockSources: scanner.MessageSource[] = [
      scanner.MessageSource.createFrom({
        path: "/path/to/file1.msg",
        in_zip: false,
        zip_path: "",
        display_name: "file1.msg",
      }),
    ];
    const mockScanResult: appservice.ScanResult = appservice.ScanResult.createFrom({
      count: 1,
      source_paths: ["/path/to/file1.msg"],
      sources: mockSources,
    });

    const globalObj = globalThis as unknown as {
      window: {
        go: {
          appservice: {
            Service: { ScanSources: (paths: string[]) => Promise<appservice.ScanResult> };
          };
        };
      };
    };
    globalObj.window.go.appservice.Service.ScanSources = async () => mockScanResult;

    const store = useParseStore();
    const result = await store.scanSources(["/path/to/file1.msg"]);

    expect(result).toEqual(mockScanResult);
    expect(store.sources).toEqual(mockSources);
    expect(store.sourcePaths).toEqual(["/path/to/file1.msg"]);
    expect(store.status).toBe("idle");
    expect(store.hasSources).toBe(true);
  });

  it("parses sources and commits records when current", async () => {
    const mockSources: scanner.MessageSource[] = [
      scanner.MessageSource.createFrom({
        path: "/path/to/file1.msg",
        in_zip: false,
        zip_path: "",
        display_name: "file1.msg",
      }),
    ];
    const mockRecords: engine.Record[] = [
      engine.Record.createFrom({
        source_file: "file1.msg",
        subject: "Tag list",
        message_date: "2026-01-02",
        reported_at: "2026-01-02 08:00",
        dispatcher: "John",
        row_in_message: 1,
        raw_entry: "123 Main St Trash Not Out",
        location: "123 Main St",
        issue: "Trash Not Out",
        label: "msw_not_out",
        issue_time: "",
      }),
    ];
    const mockParseResult: appservice.ParseResult = appservice.ParseResult.createFrom({
      records: mockRecords,
      skipped: [],
      superseded: false,
    });

    const globalObj = globalThis as unknown as {
      window: { go: { appservice: { Service: { Parse: () => Promise<appservice.ParseResult> } } } };
    };
    globalObj.window.go.appservice.Service.Parse = async () => mockParseResult;

    const store = useParseStore();
    store.sources = mockSources;

    const result = await store.parse();

    expect(result).toEqual(mockParseResult);
    expect(store.records).toEqual(mockRecords);
    expect(store.status).toBe("ready");
    expect(store.hasRecords).toBe(true);
  });

  it("suppresses stale parse Promise results after reset", async () => {
    let resolveParse: (value: appservice.ParseResult) => void = () => {};
    const parsePromise = new Promise<appservice.ParseResult>((resolve) => {
      resolveParse = resolve;
    });

    const globalObj = globalThis as unknown as {
      window: {
        go: {
          appservice: {
            Service: {
              Parse: () => Promise<appservice.ParseResult>;
              ClearParse: () => Promise<void>;
            };
          };
        };
      };
    };
    globalObj.window.go.appservice.Service.Parse = () => parsePromise;
    globalObj.window.go.appservice.Service.ClearParse = async () => {};

    const store = useParseStore();
    store.sources = [
      scanner.MessageSource.createFrom({
        path: "/test.msg",
        in_zip: false,
        zip_path: "",
        display_name: "test.msg",
      }),
    ];

    // Start parsing
    const inFlightParse = store.parse();
    expect(store.status).toBe("parsing");
    expect(store.requestEpoch).toBe(1);

    // User resets before parse completes
    await store.reset();
    expect(store.status).toBe("idle");
    expect(store.sources).toEqual([]);
    expect(store.requestEpoch).toBe(2);

    // Now resolve the old parse
    resolveParse(
      appservice.ParseResult.createFrom({
        records: [engine.Record.createFrom({ location: "Stale Loc" })],
        skipped: [],
        superseded: false,
      }),
    );

    const parseResult = await inFlightParse;
    expect(parseResult).toBeNull();
    expect(store.records).toEqual([]);
    expect(store.status).toBe("idle");
  });

  it("handles superseded: true by not committing superseded records", async () => {
    const mockParseResult: appservice.ParseResult = appservice.ParseResult.createFrom({
      records: [engine.Record.createFrom({ location: "Old Loc" })],
      skipped: [],
      superseded: true,
    });

    const globalObj = globalThis as unknown as {
      window: { go: { appservice: { Service: { Parse: () => Promise<appservice.ParseResult> } } } };
    };
    globalObj.window.go.appservice.Service.Parse = async () => mockParseResult;

    const store = useParseStore();
    store.sources = [
      scanner.MessageSource.createFrom({
        path: "/test.msg",
        in_zip: false,
        zip_path: "",
        display_name: "test.msg",
      }),
    ];

    const result = await store.parse();

    expect(result?.superseded).toBe(true);
    expect(store.records).toEqual([]);
    expect(store.status).toBe("parsing");
  });

  it("scans multiple dropped or selected files in a single ScanSources call", async () => {
    let receivedPaths: string[] = [];
    const mockScanResult: appservice.ScanResult = appservice.ScanResult.createFrom({
      count: 3,
      source_paths: ["/dir/sub", "/file1.msg", "/archive.zip"],
      sources: [
        scanner.MessageSource.createFrom({
          path: "/dir/sub/f1.msg",
          in_zip: false,
          zip_path: "",
          display_name: "f1.msg",
        }),
        scanner.MessageSource.createFrom({
          path: "/file1.msg",
          in_zip: false,
          zip_path: "",
          display_name: "file1.msg",
        }),
        scanner.MessageSource.createFrom({
          path: "inside.msg",
          in_zip: true,
          zip_path: "/archive.zip",
          display_name: "inside.msg (in archive.zip)",
        }),
      ],
    });

    const globalObj = globalThis as unknown as {
      window: {
        go: {
          appservice: {
            Service: { ScanSources: (paths: string[]) => Promise<appservice.ScanResult> };
          };
        };
      };
    };
    globalObj.window.go.appservice.Service.ScanSources = async (paths: string[]) => {
      receivedPaths = paths;
      return mockScanResult;
    };

    const store = useParseStore();
    const inputPaths = ["/dir/sub", "/file1.msg", "/archive.zip"];
    const result = await store.scanSources(inputPaths);

    expect(receivedPaths).toEqual(inputPaths);
    expect(result?.count).toBe(3);
    expect(store.sources.length).toBe(3);
  });

  it("returns null on cancelled selectFiles without mutating state", async () => {
    const globalObj = globalThis as unknown as {
      window: { go: { appservice: { Service: { SelectFiles: () => Promise<string[]> } } } };
    };
    globalObj.window.go.appservice.Service.SelectFiles = async () => [];

    const store = useParseStore();
    const result = await store.selectFiles();

    expect(result).toBeNull();
    expect(store.sources).toEqual([]);
    expect(store.status).toBe("idle");
  });

  it("returns null on cancelled selectFolder without mutating state", async () => {
    const globalObj = globalThis as unknown as {
      window: { go: { appservice: { Service: { SelectFolder: () => Promise<string> } } } };
    };
    globalObj.window.go.appservice.Service.SelectFolder = async () => "";

    const store = useParseStore();
    const result = await store.selectFolder();

    expect(result).toBeNull();
    expect(store.sources).toEqual([]);
    expect(store.status).toBe("idle");
  });

  it("handles cancelled saveAs without error or state mutation", async () => {
    const globalObj = globalThis as unknown as {
      window: { go: { appservice: { Service: { SaveAs: () => Promise<appservice.SavedFile> } } } };
    };
    globalObj.window.go.appservice.Service.SaveAs = async () =>
      appservice.SavedFile.createFrom({ cancelled: true });

    const store = useParseStore();
    const result = await store.saveAs();

    expect(result?.cancelled).toBe(true);
  });
});
