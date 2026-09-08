<script setup lang="ts">
import { ref } from "vue";
import { useParseStore } from "@/stores/parse";

const parseStore = useParseStore();
const isExpanded = ref(false);
</script>

<template>
  <div
    v-if="parseStore.hasSkipped"
    class="mb-3 rounded-xl border border-amber-300 dark:border-amber-700/80 bg-amber-50 dark:bg-amber-950/40 p-3 text-xs text-amber-900 dark:text-amber-200 transition-all shadow-xs"
  >
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-2">
        <svg
          class="w-4 h-4 text-amber-600 dark:text-amber-400 shrink-0"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
          />
        </svg>
        <span class="font-semibold">
          {{ parseStore.skipped.length }} source file{{
            parseStore.skipped.length === 1 ? "" : "s"
          }}
          could not be parsed
        </span>
      </div>

      <button
        type="button"
        class="text-amber-700 dark:text-amber-300 hover:text-amber-900 dark:hover:text-amber-100 font-medium underline cursor-pointer"
        @click="isExpanded = !isExpanded"
      >
        {{ isExpanded ? "Hide Details" : "Show Details" }}
      </button>
    </div>

    <div
      v-if="isExpanded"
      class="mt-2.5 pt-2 border-t border-amber-200 dark:border-amber-800/60 max-h-48 overflow-y-auto space-y-1.5 font-mono"
    >
      <div
        v-for="(item, idx) in parseStore.skipped"
        :key="idx"
        class="flex flex-col sm:flex-row sm:items-baseline gap-1 text-[11px] p-1.5 rounded bg-amber-100/60 dark:bg-amber-900/30"
      >
        <span class="font-semibold text-amber-950 dark:text-amber-100 break-all"
          >{{ item.display_name }}:</span
        >
        <span class="text-amber-800 dark:text-amber-300 break-words">{{ item.error }}</span>
      </div>
    </div>
  </div>
</template>
