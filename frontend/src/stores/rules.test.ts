import { describe, expect, it, beforeEach } from "bun:test";
import { setActivePinia, createPinia } from "pinia";
import { useRulesStore, cloneConfig } from "./rules";
import { appservice, config } from "wailsjs/go/models";

describe("Rules Store", () => {
  let dirtySetCalls: boolean[] = [];

  const sampleConfig: config.RuleConfig = config.RuleConfig.createFrom({
    version: 1,
    enable_heuristics: true,
    default_label: "other",
    labels: [
      config.LabelDefinition.createFrom({
        key: "msw_not_out",
        display_name: "Trash Not Out",
        metric: "trash",
      }),
      config.LabelDefinition.createFrom({
        key: "recyc_not_out",
        display_name: "Recycle Not Out",
        metric: "recycling",
      }),
      config.LabelDefinition.createFrom({ key: "other", display_name: "Other", metric: "none" }),
    ],
    rules: [
      config.ClassificationRule.createFrom({
        id: "rule_1",
        pattern: "TRASH NOT OUT",
        type: "substring",
        label: "msw_not_out",
        enabled: true,
      }),
      config.ClassificationRule.createFrom({
        id: "rule_2",
        pattern: "RCY NOT OUT",
        type: "substring",
        label: "recyc_not_out",
        enabled: true,
      }),
    ],
  });

  beforeEach(() => {
    setActivePinia(createPinia());
    dirtySetCalls = [];

    const globalObj = globalThis as unknown as {
      window: {
        go: {
          appservice: {
            Service: {
              GetRules: () => Promise<config.RuleConfig>;
              GetDefaultRules: () => Promise<config.RuleConfig>;
              SetRulesDirty: (dirty: boolean) => Promise<void>;
              ValidateRules: (cfg: config.RuleConfig) => Promise<unknown[]>;
              SaveRules: (cfg: config.RuleConfig) => Promise<void>;
            };
          };
        };
      };
    };

    globalObj.window = {
      go: {
        appservice: {
          Service: {
            GetRules: async () => cloneConfig(sampleConfig),
            GetDefaultRules: async () => cloneConfig(sampleConfig),
            SetRulesDirty: async (dirty: boolean) => {
              dirtySetCalls.push(dirty);
            },
            ValidateRules: async () => [],
            SaveRules: async () => {},
          },
        },
      },
    };
  });

  it("maintains deep-clone isolation between savedConfig and workingConfig", async () => {
    const store = useRulesStore();
    await store.loadRules();

    expect(store.savedConfig).not.toBeNull();
    expect(store.workingConfig).not.toBeNull();
    expect(store.savedConfig?.rules).not.toBe(store.workingConfig?.rules);
    expect(store.savedConfig?.labels).not.toBe(store.workingConfig?.labels);

    // Mutate workingConfig
    await store.updateRule(0, { pattern: "MODIFIED PATTERN" });

    expect(store.workingConfig?.rules[0].pattern).toBe("MODIFIED PATTERN");
    expect(store.savedConfig?.rules[0].pattern).toBe("TRASH NOT OUT");
  });

  it("handles reorder precedence correctly via moveRule up and down", async () => {
    const store = useRulesStore();
    await store.loadRules();

    expect(store.workingConfig?.rules[0].id).toBe("rule_1");
    expect(store.workingConfig?.rules[1].id).toBe("rule_2");

    // Move rule 0 down
    const movedDown = await store.moveRule(0, "down");
    expect(movedDown).toBe(true);
    expect(store.workingConfig?.rules[0].id).toBe("rule_2");
    expect(store.workingConfig?.rules[1].id).toBe("rule_1");
    expect(store.selectedIndex).toBe(1);
    expect(store.isDirty).toBe(true);

    // Move rule 1 back up
    const movedUp = await store.moveRule(1, "up");
    expect(movedUp).toBe(true);
    expect(store.workingConfig?.rules[0].id).toBe("rule_1");
    expect(store.workingConfig?.rules[1].id).toBe("rule_2");
    expect(store.selectedIndex).toBe(0);
  });

  it("rejects boundary moves at start and end of rules array", async () => {
    const store = useRulesStore();
    await store.loadRules();

    // Move index 0 up -> should fail
    const movedUp = await store.moveRule(0, "up");
    expect(movedUp).toBe(false);
    expect(store.isDirty).toBe(false);

    // Move last index down -> should fail
    const lastIndex = (store.workingConfig?.rules.length ?? 0) - 1;
    const movedDown = await store.moveRule(lastIndex, "down");
    expect(movedDown).toBe(false);
    expect(store.isDirty).toBe(false);
  });

  it("toggles rule enabled state and marks store dirty", async () => {
    const store = useRulesStore();
    await store.loadRules();

    expect(store.workingConfig?.rules[0].enabled).toBe(true);
    await store.toggleRule(0);

    expect(store.workingConfig?.rules[0].enabled).toBe(false);
    expect(store.isDirty).toBe(true);
    expect(dirtySetCalls).toContain(true);
  });

  it("enforces dirty-mirror ordering: calls SetRulesDirty(true) before mutating local state", async () => {
    let order: string[] = [];
    const globalObj = globalThis as unknown as {
      window: {
        go: { appservice: { Service: { SetRulesDirty: (dirty: boolean) => Promise<void> } } };
      };
    };
    globalObj.window.go.appservice.Service.SetRulesDirty = async (dirty: boolean) => {
      order.push(`SetRulesDirty:${dirty}`);
    };

    const store = useRulesStore();
    await store.loadRules();
    order = []; // reset order after load

    await store.addRule({ pattern: "NEW PATTERN", label: "other" });
    expect(order[0]).toBe("SetRulesDirty:true");
    expect(store.isDirty).toBe(true);
    expect(store.workingConfig?.rules.length).toBe(3);
  });

  it("rejects every config mutation while isSaving is true", async () => {
    const store = useRulesStore();
    await store.loadRules();

    store.isSaving = true;

    expect(store.addRule({ pattern: "TEST", label: "other" })).rejects.toThrow(
      "Cannot modify rules while save is in progress",
    );
    expect(store.updateRule(0, { pattern: "TEST" })).rejects.toThrow(
      "Cannot modify rules while save is in progress",
    );
    expect(store.deleteRule(0)).rejects.toThrow("Cannot modify rules while save is in progress");
    expect(store.moveRule(0, "down")).rejects.toThrow(
      "Cannot modify rules while save is in progress",
    );
    expect(store.toggleRule(0)).rejects.toThrow("Cannot modify rules while save is in progress");
    expect(store.resetDefaults()).rejects.toThrow("Cannot modify rules while save is in progress");
    expect(store.importConfig(sampleConfig)).rejects.toThrow(
      "Cannot modify rules while save is in progress",
    );
    expect(store.updateSettings({ enable_heuristics: false })).rejects.toThrow(
      "Cannot modify rules while save is in progress",
    );
    expect(
      store.addOrUpdateLabel(
        config.LabelDefinition.createFrom({ key: "new", display_name: "New", metric: "none" }),
      ),
    ).rejects.toThrow("Cannot modify rules while save is in progress");
    expect(store.deleteLabel("other")).rejects.toThrow(
      "Cannot modify rules while save is in progress",
    );
  });

  it("resets to defaults with deep-clone isolation and sets isDirty to true", async () => {
    const store = useRulesStore();
    await store.loadRules();

    await store.resetDefaults();
    expect(store.isDirty).toBe(true);
    expect(store.workingConfig).not.toBeNull();
    expect(store.selectedIndex).toBe(-1);
  });

  it("discards changes and restores workingConfig from savedConfig", async () => {
    const store = useRulesStore();
    await store.loadRules();

    await store.addRule({ pattern: "TEMP PATTERN", label: "other" });
    expect(store.workingConfig?.rules.length).toBe(3);
    expect(store.isDirty).toBe(true);

    await store.discardChanges();
    expect(store.workingConfig?.rules.length).toBe(2);
    expect(store.isDirty).toBe(false);
    expect(dirtySetCalls[dirtySetCalls.length - 1]).toBe(false);
  });

  it("handles cancelled importRulesFromFile by returning null and not mutating workingConfig", async () => {
    const globalObj = globalThis as unknown as {
      window: { go: { appservice: { Service: { ImportRules: () => Promise<unknown> } } } };
    };
    globalObj.window.go.appservice.Service.ImportRules = async () => ({ cancelled: true });

    const store = useRulesStore();
    await store.loadRules();

    const initialRuleCount = store.workingConfig?.rules.length;
    const res = await store.importRulesFromFile();

    expect(res).toBeNull();
    expect(store.workingConfig?.rules.length).toBe(initialRuleCount);
    expect(store.isDirty).toBe(false);
  });
  it("handles successful importRulesFromFile by updating workingConfig with deep-clone isolation and marking dirty", async () => {
    const importedCfg = config.RuleConfig.createFrom({
      version: 1,
      enable_heuristics: false,
      default_label: "custom_default",
      labels: [
        config.LabelDefinition.createFrom({
          key: "custom_default",
          name: "Custom Default",
          metric: "none",
        }),
      ],
      rules: [
        config.ClassificationRule.createFrom({
          id: "imported_rule_1",
          pattern: "IMPORTED PATTERN",
          type: "substring",
          label: "custom_default",
          enabled: true,
          description: "Imported rule description",
        }),
      ],
    });

    const globalObj = globalThis as unknown as {
      window: { go: { appservice: { Service: { ImportRules: () => Promise<unknown> } } } };
    };
    globalObj.window.go.appservice.Service.ImportRules = async () => ({
      cancelled: false,
      config: importedCfg,
    });

    const store = useRulesStore();
    await store.loadRules();

    const res = await store.importRulesFromFile();
    expect(res).not.toBeNull();
    expect(store.workingConfig?.rules.length).toBe(1);
    expect(store.workingConfig?.rules[0].pattern).toBe("IMPORTED PATTERN");
    expect(store.workingConfig?.default_label).toBe("custom_default");
    expect(store.isDirty).toBe(true);
    expect(dirtySetCalls[dirtySetCalls.length - 1]).toBe(true);

    // Deep clone isolation check: mutating importedCfg object should not mutate workingConfig
    importedCfg.rules[0].pattern = "MUTATED";
    expect(store.workingConfig?.rules[0].pattern).toBe("IMPORTED PATTERN");
  });
  it("handles cancelled exportRulesToFile returning cancelled result without error", async () => {
    const globalObj = globalThis as unknown as {
      window: { go: { appservice: { Service: { ExportRules: () => Promise<unknown> } } } };
    };
    globalObj.window.go.appservice.Service.ExportRules = async () =>
      appservice.SavedFile.createFrom({ cancelled: true });

    const store = useRulesStore();
    await store.loadRules();

    const res = await store.exportRulesToFile();
    expect(res).toEqual(appservice.SavedFile.createFrom({ cancelled: true }));
    expect(store.isDirty).toBe(false);
  });
});
