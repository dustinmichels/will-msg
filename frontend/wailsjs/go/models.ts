export namespace appservice {
  export class ImportRulesResult {
    config?: config.RuleConfig;
    cancelled: boolean;

    static createFrom(source: any = {}) {
      return new ImportRulesResult(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.config = this.convertValues(source["config"], config.RuleConfig);
      this.cancelled = source["cancelled"];
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }
  export class SkippedSource {
    display_name: string;
    error: string;

    static createFrom(source: any = {}) {
      return new SkippedSource(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.display_name = source["display_name"];
      this.error = source["error"];
    }
  }
  export class ParseResult {
    records: engine.Record[];
    skipped: SkippedSource[];
    superseded: boolean;

    static createFrom(source: any = {}) {
      return new ParseResult(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.records = this.convertValues(source["records"], engine.Record);
      this.skipped = this.convertValues(source["skipped"], SkippedSource);
      this.superseded = source["superseded"];
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }
  export class SandboxResult {
    address: string;
    status: string;
    issue_time: string;
    label: string;
    matched_rule_index: number;
    matched_rule_id: string;
    match_kind: string;
    metric: string;

    static createFrom(source: any = {}) {
      return new SandboxResult(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.address = source["address"];
      this.status = source["status"];
      this.issue_time = source["issue_time"];
      this.label = source["label"];
      this.matched_rule_index = source["matched_rule_index"];
      this.matched_rule_id = source["matched_rule_id"];
      this.match_kind = source["match_kind"];
      this.metric = source["metric"];
    }
  }
  export class SavedFile {
    filename: string;
    dir: string;
    path: string;
    cancelled: boolean;

    static createFrom(source: any = {}) {
      return new SavedFile(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.filename = source["filename"];
      this.dir = source["dir"];
      this.path = source["path"];
      this.cancelled = source["cancelled"];
    }
  }
  export class ScanResult {
    sources: scanner.MessageSource[];
    count: number;
    source_paths: string[];

    static createFrom(source: any = {}) {
      return new ScanResult(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.sources = this.convertValues(source["sources"], scanner.MessageSource);
      this.count = source["count"];
      this.source_paths = source["source_paths"];
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }

  export class ValidationError {
    path: string;
    message: string;

    static createFrom(source: any = {}) {
      return new ValidationError(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.path = source["path"];
      this.message = source["message"];
    }
  }
}

export namespace config {
  export class ClassificationRule {
    id: string;
    pattern: string;
    type: string;
    label: string;
    description?: string;
    enabled: boolean;

    static createFrom(source: any = {}) {
      return new ClassificationRule(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.id = source["id"];
      this.pattern = source["pattern"];
      this.type = source["type"];
      this.label = source["label"];
      this.description = source["description"];
      this.enabled = source["enabled"];
    }
  }
  export class LabelDefinition {
    key: string;
    display_name: string;
    metric: string;

    static createFrom(source: any = {}) {
      return new LabelDefinition(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.key = source["key"];
      this.display_name = source["display_name"];
      this.metric = source["metric"];
    }
  }
  export class RuleConfig {
    version: number;
    enable_heuristics: boolean;
    default_label: string;
    labels: LabelDefinition[];
    rules: ClassificationRule[];

    static createFrom(source: any = {}) {
      return new RuleConfig(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.version = source["version"];
      this.enable_heuristics = source["enable_heuristics"];
      this.default_label = source["default_label"];
      this.labels = this.convertValues(source["labels"], LabelDefinition);
      this.rules = this.convertValues(source["rules"], ClassificationRule);
    }

    convertValues(a: any, classs: any, asMap: boolean = false): any {
      if (!a) {
        return a;
      }
      if (a.slice && a.map) {
        return (a as any[]).map((elem) => this.convertValues(elem, classs));
      } else if ("object" === typeof a) {
        if (asMap) {
          for (const key of Object.keys(a)) {
            a[key] = new classs(a[key]);
          }
          return a;
        }
        return new classs(a);
      }
      return a;
    }
  }
}

export namespace engine {
  export class Record {
    source_file: string;
    subject: string;
    message_date: string;
    reported_at: string;
    dispatcher: string;
    row_in_message: number;
    raw_entry: string;
    location: string;
    issue: string;
    label: string;
    issue_time: string;

    static createFrom(source: any = {}) {
      return new Record(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.source_file = source["source_file"];
      this.subject = source["subject"];
      this.message_date = source["message_date"];
      this.reported_at = source["reported_at"];
      this.dispatcher = source["dispatcher"];
      this.row_in_message = source["row_in_message"];
      this.raw_entry = source["raw_entry"];
      this.location = source["location"];
      this.issue = source["issue"];
      this.label = source["label"];
      this.issue_time = source["issue_time"];
    }
  }
}

export namespace scanner {
  export class MessageSource {
    path: string;
    in_zip: boolean;
    zip_path: string;
    display_name: string;

    static createFrom(source: any = {}) {
      return new MessageSource(source);
    }

    constructor(source: any = {}) {
      if ("string" === typeof source) source = JSON.parse(source);
      this.path = source["path"];
      this.in_zip = source["in_zip"];
      this.zip_path = source["zip_path"];
      this.display_name = source["display_name"];
    }
  }
}
