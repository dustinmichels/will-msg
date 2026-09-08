<script setup lang="ts">
import { useParseStore } from "@/stores/parse";
import RunParserButton from "@/components/RunParserButton.vue";

const parseStore = useParseStore();

async function handleLoadNewSource() {
  await parseStore.reset();
}
</script>

<template>
  <div
    class="flex flex-col h-full bg-white dark:bg-slate-850 border-r border-slate-200 dark:border-slate-800 select-none"
  >
    <!-- Top Area -->
    <div class="p-4 border-b border-slate-200 dark:border-slate-800 shrink-0 space-y-3">
      <!-- Back / Load New Source Button -->
      <button
        type="button"
        :disabled="parseStore.isProcessing"
        class="inline-flex items-center gap-2 px-3 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 bg-slate-100 dark:bg-slate-750 hover:bg-slate-200 dark:hover:bg-slate-700 border border-slate-300 dark:border-slate-650 rounded-lg transition-colors cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
        @click="handleLoadNewSource"
      >
        <svg
          class="w-4 h-4 text-slate-500 dark:text-slate-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6"
          />
        </svg>
        <span>Load New Source</span>
      </button>

      <!-- Selected Source Info -->
      <div>
        <h3
          class="text-xs font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400 mb-1"
        >
          Selected MSG Source:
        </h3>
        <div
          class="text-xs font-mono text-slate-800 dark:text-slate-200 bg-slate-50 dark:bg-slate-900/60 p-2 rounded-md border border-slate-200/60 dark:border-slate-800 break-all max-h-20 overflow-y-auto"
        >
          <div
            v-for="(path, idx) in parseStore.sourcePaths"
            :key="idx"
            class="truncate"
            :title="path"
          >
            {{ path }}
          </div>
          <div v-if="parseStore.sourcePaths.length === 0" class="italic text-slate-400">
            Selected: None
          </div>
        </div>
      </div>

      <!-- File Count -->
      <div class="text-xs italic text-slate-600 dark:text-slate-400">
        Found {{ parseStore.sources.length }} .msg file{{
          parseStore.sources.length === 1 ? "" : "s"
        }}
      </div>

      <div class="pt-1 flex items-center justify-between">
        <h4 class="text-xs font-bold text-slate-700 dark:text-slate-300">Detected Files:</h4>
        <span class="text-[11px] font-mono text-slate-400">
          {{ parseStore.sources.length }} items
        </span>
      </div>
    </div>

    <!-- Scrollable Detected Files List -->
    <div class="flex-1 overflow-y-auto p-2 divide-y divide-slate-100 dark:divide-slate-800/60">
      <div
        v-for="(source, idx) in parseStore.sources"
        :key="idx"
        class="flex items-center gap-2 px-2.5 py-1.5 text-xs rounded-md hover:bg-slate-100 dark:hover:bg-slate-800/80 transition-colors group"
        :title="source.display_name"
      >
        <!-- Document / Zip Icon -->
        <svg
          v-if="source.in_zip"
          class="w-4 h-4 text-amber-500 shrink-0"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4"
          />
        </svg>
        <svg
          v-else
          class="w-4 h-4 text-truck-600 dark:text-truck-400 shrink-0"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
          />
        </svg>

        <span
          class="truncate flex-1 font-mono text-slate-700 dark:text-slate-300 group-hover:text-slate-900 dark:group-hover:text-white"
        >
          {{ source.display_name }}
        </span>
      </div>

      <div
        v-if="parseStore.sources.length === 0"
        class="p-4 text-center text-xs text-slate-400 italic"
      >
        No files detected
      </div>
    </div>

    <!-- Bottom Action Area -->
    <div
      class="p-3 border-t border-slate-200 dark:border-slate-800 shrink-0 bg-slate-50 dark:bg-slate-900/40"
    >
      <RunParserButton />
    </div>
  </div>
</template>
