<script setup lang="ts">
import { useParseStore } from "@/stores/parse";
import { toast } from "@/lib/toast";

const parseStore = useParseStore();

async function handleSelectFiles() {
  try {
    const res = await parseStore.selectFiles();
    if (res && res.count === 0) {
      toast.warning("No .msg files were found in the selected source.", "No Files Found");
    }
  } catch (err: unknown) {
    toast.error(err, "File Selection Error");
  }
}

async function handleSelectFolder() {
  try {
    const res = await parseStore.selectFolder();
    if (res && res.count === 0) {
      toast.warning("No .msg files were found in the selected source.", "No Files Found");
    }
  } catch (err: unknown) {
    toast.error(err, "Folder Selection Error");
  }
}
</script>

<template>
  <div class="flex-1 flex items-center justify-center p-6 md:p-12 overflow-y-auto">
    <div
      style="--wails-drop-target: drop"
      class="welcome-drop-zone w-full max-w-2xl min-h-[380px] sm:min-h-[420px] rounded-2xl border-2 border-truck-500 bg-truck-100 dark:bg-slate-800/90 shadow-lg p-8 flex flex-col items-center justify-center text-center transition-all duration-200"
    >
      <!-- Upload Icon -->
      <div
        class="w-20 h-20 mb-4 rounded-full bg-truck-500/10 dark:bg-truck-500/20 text-truck-500 flex items-center justify-center"
      >
        <svg class="w-10 h-10" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"
          />
        </svg>
      </div>

      <!-- Headline -->
      <h2 class="text-welcome font-bold text-truck-500 mb-2 tracking-tight">
        Feed me your msg files, Will
      </h2>

      <!-- Description -->
      <p class="text-sm sm:text-base text-slate-600 dark:text-slate-300 italic max-w-md mb-8">
        Drop or select. Accepts .msg files, folders containing .msg files, or .zip archives.
      </p>

      <!-- Action Buttons -->
      <div class="flex flex-wrap items-center justify-center gap-4">
        <button
          type="button"
          :disabled="parseStore.isProcessing"
          class="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl font-semibold text-white bg-truck-500 hover:bg-truck-600 active:bg-truck-600 shadow-md hover:shadow-lg focus:outline-hidden focus:ring-2 focus:ring-truck-500 focus:ring-offset-2 transition-all cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
          @click="handleSelectFiles"
        >
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
            />
          </svg>
          <span>Choose files…</span>
        </button>

        <button
          type="button"
          :disabled="parseStore.isProcessing"
          class="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl font-semibold text-slate-700 dark:text-slate-200 bg-white dark:bg-slate-700 hover:bg-slate-50 dark:hover:bg-slate-600 border border-slate-300 dark:border-slate-600 shadow-xs hover:shadow-md focus:outline-hidden focus:ring-2 focus:ring-truck-500 focus:ring-offset-2 transition-all cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
          @click="handleSelectFolder"
        >
          <svg
            class="w-5 h-5 text-truck-600 dark:text-truck-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"
            />
          </svg>
          <span>Choose folder…</span>
        </button>
      </div>

      <!-- Scanning Indicator -->
      <div
        v-if="parseStore.isScanning"
        class="mt-6 flex items-center gap-2 text-sm text-truck-600 dark:text-truck-400 font-medium"
      >
        <svg class="animate-spin h-5 w-5" viewBox="0 0 24 24" fill="none">
          <circle
            class="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            stroke-width="4"
          ></circle>
          <path
            class="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
          ></path>
        </svg>
        <span>Scanning sources for .msg files…</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.welcome-drop-zone.wails-drop-target-active {
  outline: 4px solid var(--color-truck-500, #6cb944);
  outline-offset: 4px;
  transform: scale(1.02);
  background-color: var(--color-truck-200, #dcf5c3);
}
</style>
