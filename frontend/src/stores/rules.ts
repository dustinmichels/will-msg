import { defineStore } from "pinia";
import type { appservice, config } from "wailsjs/go/models";
import * as Service from "wailsjs/go/appservice/Service";

export interface RulesState {
  savedConfig: config.RuleConfig | null;
  workingConfig: config.RuleConfig | null;
  selectedIndex: number;
  isDirty: boolean;
  isSaving: boolean;
  validation: appservice.ValidationError[];
  isLoading: boolean;
}

export function cloneRule(rule: config.ClassificationRule): config.ClassificationRule {
  return {
    id: rule.id,
    pattern: rule.pattern,
    type: rule.type,
    label: rule.label,
    description: rule.description,
    enabled: rule.enabled,
  } as config.ClassificationRule;
}

export function cloneLabel(label: config.LabelDefinition): config.LabelDefinition {
  return {
    key: label.key,
    display_name: label.display_name,
    metric: label.metric,
  } as config.LabelDefinition;
}

export function cloneConfig(cfg: config.RuleConfig): config.RuleConfig {
  return {
    version: cfg.version,
    enable_heuristics: cfg.enable_heuristics,
    default_label: cfg.default_label,
    labels: (cfg.labels || []).map(cloneLabel),
    rules: (cfg.rules || []).map(cloneRule),
  } as config.RuleConfig;
}

export function generateRuleId(): string {
  return `rule_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}

export const useRulesStore = defineStore("rules", {
  state: (): RulesState => ({
    savedConfig: null,
    workingConfig: null,
    selectedIndex: -1,
    isDirty: false,
    isSaving: false,
    validation: [],
    isLoading: false,
  }),

  getters: {
    selectedRule: (state): config.ClassificationRule | null => {
      if (
        !state.workingConfig ||
        state.selectedIndex < 0 ||
        state.selectedIndex >= state.workingConfig.rules.length
      ) {
        return null;
      }
      return state.workingConfig.rules[state.selectedIndex];
    },
    rulesCount: (state): number => state.workingConfig?.rules.length ?? 0,
    canMoveUp: (state): boolean => state.selectedIndex > 0,
    canMoveDown: (state): boolean => {
      if (!state.workingConfig) return false;
      return state.selectedIndex >= 0 && state.selectedIndex < state.workingConfig.rules.length - 1;
    },
    hasSelection: (state): boolean => state.selectedIndex >= 0,
  },

  actions: {
    async ensureDirty(): Promise<void> {
      if (this.isSaving) {
        throw new Error("Cannot modify rules while save is in progress");
      }
      if (!this.isDirty) {
        await Service.SetRulesDirty(true);
        this.isDirty = true;
      }
    },

    async clearDirty(): Promise<void> {
      await Service.SetRulesDirty(false);
      this.isDirty = false;
    },

    async loadRules(): Promise<config.RuleConfig> {
      this.isLoading = true;
      try {
        const cfg = await Service.GetRules();
        this.savedConfig = cloneConfig(cfg);
        this.workingConfig = cloneConfig(cfg);
        this.selectedIndex = -1;
        this.isDirty = false;
        this.validation = [];
        await Service.SetRulesDirty(false);
        return this.workingConfig;
      } finally {
        this.isLoading = false;
      }
    },

    async addRule(
      rule: Partial<config.ClassificationRule> & { pattern: string; label: string },
    ): Promise<void> {
      if (this.isSaving) {
        throw new Error("Cannot modify rules while save is in progress");
      }
      await this.ensureDirty();
      if (!this.workingConfig) {
        return;
      }

      const newRule: config.ClassificationRule = {
        id: rule.id || generateRuleId(),
        pattern: rule.pattern,
        type: rule.type || "substring",
        label: rule.label,
        description: rule.description || "",
        enabled: rule.enabled ?? true,
      } as config.ClassificationRule;

      this.workingConfig.rules.push(cloneRule(newRule));
      this.selectedIndex = this.workingConfig.rules.length - 1;
    },

    async updateRule(index: number, update: Partial<config.ClassificationRule>): Promise<void> {
      if (this.isSaving) {
        throw new Error("Cannot modify rules while save is in progress");
      }
      if (!this.workingConfig || index < 0 || index >= this.workingConfig.rules.length) {
        return;
      }
      await this.ensureDirty();

      const current = this.workingConfig.rules[index];
      this.workingConfig.rules[index] = {
        ...current,
        ...update,
      } as config.ClassificationRule;
    },

    async deleteRule(index: number): Promise<void> {
      if (this.isSaving) {
        throw new Error("Cannot modify rules while save is in progress");
      }
      if (!this.workingConfig || index < 0 || index >= this.workingConfig.rules.length) {
        return;
      }
      await this.ensureDirty();

      this.workingConfig.rules.splice(index, 1);
      if (this.workingConfig.rules.length === 0) {
        this.selectedIndex = -1;
      } else if (this.selectedIndex >= this.workingConfig.rules.length) {
        this.selectedIndex = this.workingConfig.rules.length - 1;
      } else if (this.selectedIndex === index) {
        this.selectedIndex = Math.min(index, this.workingConfig.rules.length - 1);
      }
    },

    async moveRule(index: number, direction: "up" | "down"): Promise<boolean> {
      if (this.isSaving) {
        throw new Error("Cannot modify rules while save is in progress");
      }
      if (!this.workingConfig) {
        return false;
      }
      const targetIndex = direction === "up" ? index - 1 : index + 1;
      if (
        index < 0 ||
        index >= this.workingConfig.rules.length ||
        targetIndex < 0 ||
        targetIndex >= this.workingConfig.rules.length
      ) {
        return false;
      }
      await this.ensureDirty();

      const temp = this.workingConfig.rules[index];
      this.workingConfig.rules[index] = this.workingConfig.rules[targetIndex];
      this.workingConfig.rules[targetIndex] = temp;
      this.selectedIndex = targetIndex;
      return true;
    },

    async toggleRule(index: number): Promise<void> {
      if (this.isSaving) {
        throw new Error("Cannot modify rules while save is in progress");
      }
      if (!this.workingConfig || index < 0 || index >= this.workingConfig.rules.length) {
        return;
      }
      await this.ensureDirty();

      this.workingConfig.rules[index].enabled = !this.workingConfig.rules[index].enabled;
    },

    async resetDefaults(): Promise<void> {
      if (this.isSaving) {
        throw new Error("Cannot modify rules while save is in progress");
      }
      const defaultCfg = await Service.GetDefaultRules();
      await this.ensureDirty();

      this.workingConfig = cloneConfig(defaultCfg);
      this.selectedIndex = -1;
      this.validation = [];
    },

    async importConfig(newConfig: config.RuleConfig): Promise<void> {
      if (this.isSaving) {
        throw new Error("Cannot modify rules while save is in progress");
      }
      await this.ensureDirty();

      this.workingConfig = cloneConfig(newConfig);
      this.selectedIndex = -1;
      this.validation = [];
    },

    async importRulesFromFile(): Promise<appservice.ImportRulesResult | null> {
      if (this.isSaving) {
        throw new Error("Cannot modify rules while save is in progress");
      }
      const res = await Service.ImportRules();
      if (res.cancelled || !res.config) {
        return null;
      }
      await this.importConfig(res.config);
      return res;
    },

    async exportRulesToFile(): Promise<appservice.SavedFile | null> {
      if (!this.workingConfig) {
        return null;
      }
      return await Service.ExportRules(this.workingConfig);
    },

    async updateSettings(settings: {
      enable_heuristics?: boolean;
      default_label?: string;
    }): Promise<void> {
      if (this.isSaving) {
        throw new Error("Cannot modify rules while save is in progress");
      }
      if (!this.workingConfig) {
        return;
      }
      await this.ensureDirty();

      if (settings.enable_heuristics !== undefined) {
        this.workingConfig.enable_heuristics = settings.enable_heuristics;
      }
      if (settings.default_label !== undefined) {
        this.workingConfig.default_label = settings.default_label;
      }
    },

    async addOrUpdateLabel(label: config.LabelDefinition): Promise<void> {
      if (this.isSaving) {
        throw new Error("Cannot modify rules while save is in progress");
      }
      if (!this.workingConfig) {
        return;
      }
      await this.ensureDirty();

      const idx = this.workingConfig.labels.findIndex((l) => l.key === label.key);
      if (idx >= 0) {
        this.workingConfig.labels[idx] = cloneLabel(label);
      } else {
        this.workingConfig.labels.push(cloneLabel(label));
      }
    },

    async deleteLabel(key: string): Promise<void> {
      if (this.isSaving) {
        throw new Error("Cannot modify rules while save is in progress");
      }
      if (!this.workingConfig) {
        return;
      }
      await this.ensureDirty();

      const idx = this.workingConfig.labels.findIndex((l) => l.key === key);
      if (idx >= 0) {
        this.workingConfig.labels.splice(idx, 1);
      }
    },

    async validate(): Promise<appservice.ValidationError[]> {
      if (!this.workingConfig) {
        this.validation = [];
        return [];
      }
      const errors = await Service.ValidateRules(this.workingConfig);
      this.validation = errors || [];
      return this.validation;
    },

    async saveAndApply(): Promise<boolean> {
      if (!this.workingConfig) {
        return false;
      }
      this.isSaving = true;
      try {
        const errors = await Service.ValidateRules(this.workingConfig);
        this.validation = errors || [];
        if (this.validation.length > 0) {
          throw new Error("Validation failed: " + this.validation.map((e) => e.message).join("; "));
        }
        await Service.SaveRules(this.workingConfig);
        this.savedConfig = cloneConfig(this.workingConfig);
        this.isDirty = false;
        this.validation = [];
        return true;
      } finally {
        this.isSaving = false;
      }
    },

    async discardChanges(): Promise<void> {
      if (this.savedConfig) {
        this.workingConfig = cloneConfig(this.savedConfig);
      }
      this.selectedIndex = -1;
      this.validation = [];
      await this.clearDirty();
    },

    selectRule(index: number): void {
      this.selectedIndex = index;
    },

    async testRule(input: string): Promise<appservice.SandboxResult | null> {
      if (!this.workingConfig) {
        return null;
      }
      return await Service.TestRule(input, this.workingConfig);
    },
  },
});
