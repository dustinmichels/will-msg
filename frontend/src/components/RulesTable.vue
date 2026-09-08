<script setup lang="ts">
import { useRulesStore } from "@/stores/rules";
import type { config } from "wailsjs/go/models";

const rulesStore = useRulesStore();

const emit = defineEmits<{
  (e: "edit", index: number): void;
}>();

function getMetricForLabel(labelKey: string): string {
  if (!rulesStore.workingConfig) return "none";
  const def = rulesStore.workingConfig.labels.find((l) => l.key === labelKey);
  return def ? def.metric : "none";
}

function selectRow(index: number) {
  rulesStore.selectRule(index);
}

function handleDblClick(index: number) {
  rulesStore.selectRule(index);
  emit("edit", index);
}
</script>

<template>
  <div
    class="flex-1 flex flex-col min-h-0 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-lg overflow-hidden select-none"
  >
    <div class="flex-1 overflow-auto">
      <table class="w-full text-left text-xs border-collapse">
        <!-- Sticky Header Row -->
        <thead
          class="sticky top-0 z-10 bg-truck-200 dark:bg-slate-800 text-slate-800 dark:text-slate-100 font-bold shadow-xs"
        >
          <tr>
            <th
              class="px-2 py-2 border-b border-r border-slate-300 dark:border-slate-700 text-center font-bold"
              style="width: 45px; min-width: 45px"
            >
              #
            </th>
            <th
              class="px-3 py-2 border-b border-r border-slate-300 dark:border-slate-700 font-bold"
              style="width: 95px; min-width: 95px"
            >
              Type
            </th>
            <th
              class="px-3 py-2 border-b border-r border-slate-300 dark:border-slate-700 font-bold"
              style="width: 260px; min-width: 260px"
            >
              Pattern
            </th>
            <th
              class="px-3 py-2 border-b border-r border-slate-300 dark:border-slate-700 font-bold"
              style="width: 190px; min-width: 190px"
            >
              Target Label
            </th>
            <th
              class="px-3 py-2 border-b border-r border-slate-300 dark:border-slate-700 font-bold"
              style="width: 95px; min-width: 95px"
            >
              Metric
            </th>
            <th
              class="px-3 py-2 border-b border-r border-slate-300 dark:border-slate-700 text-center font-bold"
              style="width: 75px; min-width: 75px"
            >
              Enabled
            </th>
            <th
              class="px-3 py-2 border-b border-slate-300 dark:border-slate-700 font-bold"
              style="min-width: 200px"
            >
              Description
            </th>
          </tr>
        </thead>

        <!-- Body Rows -->
        <tbody
          class="divide-y divide-slate-200 dark:divide-slate-800 text-slate-700 dark:text-slate-300"
        >
          <tr
            v-for="(rule, idx) in rulesStore.workingConfig?.rules || []"
            :key="rule.id || idx"
            class="cursor-pointer transition-colors"
            :class="[
              idx === rulesStore.selectedIndex
                ? 'bg-truck-selection dark:bg-truck-500/30 text-slate-900 dark:text-white font-bold'
                : 'hover:bg-slate-50 dark:hover:bg-slate-800/60 even:bg-slate-50/40 dark:even:bg-slate-850/40',
              !rule.enabled ? 'italic text-slate-500 dark:text-slate-400' : '',
            ]"
            @click="selectRow(idx)"
            @dblclick="handleDblClick(idx)"
          >
            <!-- # Index -->
            <td
              class="px-2 py-1.5 border-r border-slate-200 dark:border-slate-800 text-center font-mono tabular-nums text-[11px]"
            >
              {{ idx + 1 }}
            </td>

            <!-- Type -->
            <td
              class="px-3 py-1.5 border-r border-slate-200 dark:border-slate-800 font-mono text-[11px]"
            >
              {{ rule.type }}
            </td>

            <!-- Pattern -->
            <td
              class="px-3 py-1.5 border-r border-slate-200 dark:border-slate-800 font-mono text-[11px] truncate max-w-[260px]"
              :title="rule.pattern"
            >
              {{ rule.pattern }}
            </td>

            <!-- Target Label -->
            <td
              class="px-3 py-1.5 border-r border-slate-200 dark:border-slate-800 font-mono text-[11px] truncate max-w-[190px]"
              :title="rule.label"
            >
              {{ rule.label }}
            </td>

            <!-- Metric -->
            <td class="px-3 py-1.5 border-r border-slate-200 dark:border-slate-800 truncate">
              {{ getMetricForLabel(rule.label) }}
            </td>

            <!-- Enabled -->
            <td
              class="px-3 py-1.5 border-r border-slate-200 dark:border-slate-800 text-center font-medium"
            >
              <span v-if="rule.enabled" class="text-truck-600 dark:text-truck-400">✓ Yes</span>
              <span v-else class="text-red-500 dark:text-red-400">✗ No</span>
            </td>

            <!-- Description -->
            <td class="px-3 py-1.5 truncate max-w-[200px]" :title="rule.description">
              {{ rule.description }}
            </td>
          </tr>

          <!-- Empty State -->
          <tr v-if="!rulesStore.workingConfig || rulesStore.workingConfig.rules.length === 0">
            <td colspan="7" class="px-6 py-12 text-center text-slate-400 italic">
              No classification rules defined. Click "Add Rule" or "Reset Defaults".
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
