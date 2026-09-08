<script setup lang="ts">
import { ref, watch, onMounted } from "vue";
import { useRulesStore } from "@/stores/rules";
import type { appservice } from "wailsjs/go/models";

const rulesStore = useRulesStore();

const inputText = ref("45 FOREST ST TRASH AND RCY NOT OUT 0830AM");
const result = ref<appservice.SandboxResult | null>(null);
const isLoading = ref(false);

let debounceTimer: ReturnType<typeof setTimeout> | null = null;

async function runSandbox() {
  const text = inputText.value.trim();
  if (!text) {
    result.value = null;
    return;
  }

  isLoading.value = true;
  try {
    const res = await rulesStore.testRule(text);
    result.value = res;
  } catch (err: unknown) {
    console.error("Sandbox evaluation failed:", err);
  } finally {
    isLoading.value = false;
  }
}

function scheduleRunSandbox() {
  if (debounceTimer) {
    clearTimeout(debounceTimer);
  }
  debounceTimer = setTimeout(() => {
    runSandbox();
  }, 150);
}

watch(inputText, () => {
  scheduleRunSandbox();
});

watch(
  () => rulesStore.workingConfig,
  () => {
    scheduleRunSandbox();
  },
  { deep: true },
);

onMounted(() => {
  runSandbox();
});

const samples = [
  { label: "Sample: Both Not Out", text: "45 FOREST ST TRASH AND RCY NOT OUT 0830AM" },
  { label: "Sample: Contaminated", text: "14 DARTMOUTH ST RECYC CONTAM" },
  { label: "Sample: Blocked", text: "88 BOSTON AVE BLOCKED ACCESS" },
  { label: "Sample: Heuristic Suffix", text: "10 HIGH ST MATTRESS NOT OUT" },
];

function setSample(text: string) {
  inputText.value = text;
}

function clearInput() {
  inputText.value = "";
  result.value = null;
}

function formatMatchedRule(): string {
  if (!inputText.value.trim()) {
    return "(Enter text above)";
  }
  if (!result.value) {
    return "-";
  }

  if (result.value.match_kind === "rule" && result.value.matched_rule_index >= 0) {
    const rule = rulesStore.workingConfig?.rules[result.value.matched_rule_index];
    if (rule) {
      return `Rule #${result.value.matched_rule_index + 1}: ${rule.pattern} (${rule.type})`;
    }
    return `Rule #${result.value.matched_rule_index + 1}`;
  }

  if (result.value.match_kind === "heuristic") {
    return "Dynamic Heuristic (*_not_out)";
  }

  return `Default Fallback (${rulesStore.workingConfig?.default_label || "unknown"})`;
}

function formatMetricImpact(): string {
  if (!inputText.value.trim()) {
    return "-";
  }
  if (!result.value) {
    return "-";
  }

  switch (result.value.metric) {
    case "trash":
      return "Trash";
    case "recycling":
      return "Recycling";
    case "both":
      return "Both (Trash + Recycling)";
    default:
      return "None";
  }
}
</script>

<template>
  <div
    class="rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-850 p-4 space-y-3 shadow-xs select-none"
  >
    <!-- Header -->
    <div>
      <h3 class="text-xs font-bold text-slate-900 dark:text-slate-100 flex items-center gap-2">
        <span>Live Rule Tester / Sandbox</span>
        <span
          v-if="isLoading"
          class="text-[10px] font-normal text-truck-600 dark:text-truck-400 animate-pulse"
          >Evaluating…</span
        >
      </h3>
      <p class="text-[11px] text-slate-500 dark:text-slate-400 italic">
        Type or paste any raw message line to verify address splitting, matching rule precedence,
        and assigned label in real time:
      </p>
    </div>

    <!-- Input Entry -->
    <div>
      <input
        v-model="inputText"
        type="text"
        placeholder="e.g. 45 FOREST ST TRASH AND RCY NOT OUT 0830AM"
        class="w-full px-3 py-2 text-xs font-mono rounded-lg border border-slate-300 dark:border-slate-700 bg-slate-50 dark:bg-slate-900 text-slate-900 dark:text-slate-100 focus:outline-hidden focus:ring-2 focus:ring-truck-500"
      />
    </div>

    <!-- Quick Samples Toolbar -->
    <div class="flex flex-wrap items-center gap-2 text-xs">
      <span class="text-[11px] font-bold text-slate-500 dark:text-slate-400 mr-1"
        >Quick Samples:</span
      >
      <button
        v-for="(s, idx) in samples"
        :key="idx"
        type="button"
        class="px-2.5 py-1 text-[11px] font-medium rounded-md bg-slate-100 dark:bg-slate-750 hover:bg-slate-200 dark:hover:bg-slate-700 text-slate-700 dark:text-slate-200 border border-slate-300/80 dark:border-slate-650 transition-colors cursor-pointer"
        @click="setSample(s.text)"
      >
        {{ s.label }}
      </button>

      <div class="flex-1"></div>

      <button
        type="button"
        class="px-2.5 py-1 text-[11px] font-medium rounded-md bg-white dark:bg-slate-800 hover:bg-red-50 dark:hover:bg-red-950/40 text-red-600 dark:text-red-400 border border-red-200 dark:border-red-900 transition-colors cursor-pointer"
        @click="clearInput"
      >
        Clear
      </button>
    </div>

    <!-- Results 6-card Grid -->
    <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-2.5 pt-1">
      <!-- 1. Detected Address -->
      <div
        class="p-2.5 rounded-lg bg-slate-50 dark:bg-slate-900/60 border border-slate-200/80 dark:border-slate-800"
      >
        <div
          class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-0.5"
        >
          Detected Address:
        </div>
        <div
          class="text-xs font-bold font-mono text-slate-800 dark:text-slate-200 truncate"
          :title="result?.address"
        >
          {{ inputText.trim() ? result?.address || "(none detected)" : "-" }}
        </div>
      </div>

      <!-- 2. Detected Issue/Status -->
      <div
        class="p-2.5 rounded-lg bg-slate-50 dark:bg-slate-900/60 border border-slate-200/80 dark:border-slate-800"
      >
        <div
          class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-0.5"
        >
          Detected Issue/Status:
        </div>
        <div
          class="text-xs font-bold font-mono text-slate-800 dark:text-slate-200 truncate"
          :title="result?.status"
        >
          {{ inputText.trim() ? result?.status || "(none detected)" : "-" }}
        </div>
      </div>

      <!-- 3. Issue Time -->
      <div
        class="p-2.5 rounded-lg bg-slate-50 dark:bg-slate-900/60 border border-slate-200/80 dark:border-slate-800"
      >
        <div
          class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-0.5"
        >
          Issue Time:
        </div>
        <div class="text-xs font-bold font-mono tabular-nums text-slate-800 dark:text-slate-200">
          {{ inputText.trim() ? result?.issue_time || "(none)" : "-" }}
        </div>
      </div>

      <!-- 4. Assigned Label -->
      <div
        class="p-2.5 rounded-lg bg-slate-50 dark:bg-slate-900/60 border border-slate-200/80 dark:border-slate-800"
      >
        <div
          class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-0.5"
        >
          Assigned Label:
        </div>
        <div
          class="text-xs font-bold font-mono text-truck-600 dark:text-truck-400 truncate"
          :title="result?.label"
        >
          {{ inputText.trim() ? result?.label || "-" : "-" }}
        </div>
      </div>

      <!-- 5. Matched Rule -->
      <div
        class="p-2.5 rounded-lg bg-slate-50 dark:bg-slate-900/60 border border-slate-200/80 dark:border-slate-800"
      >
        <div
          class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-0.5"
        >
          Matched Rule:
        </div>
        <div
          class="text-xs font-bold text-slate-800 dark:text-slate-200 truncate"
          :title="formatMatchedRule()"
        >
          {{ formatMatchedRule() }}
        </div>
      </div>

      <!-- 6. Metric Impact -->
      <div
        class="p-2.5 rounded-lg bg-slate-50 dark:bg-slate-900/60 border border-slate-200/80 dark:border-slate-800"
      >
        <div
          class="text-[10px] font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-0.5"
        >
          Metric Impact:
        </div>
        <div class="text-xs font-bold text-slate-800 dark:text-slate-200">
          {{ formatMetricImpact() }}
        </div>
      </div>
    </div>
  </div>
</template>
