<script setup lang="ts">
import { ref } from "vue";
import { useParseStore } from "@/stores/parse";
import { toast } from "@/lib/toast";
import type { appservice } from "wailsjs/go/models";
import Modal from "@/components/Modal.vue";

const parseStore = useParseStore();

const savedFileModalOpen = ref(false);
const savedFileInfo = ref<appservice.SavedFile | null>(null);
const savedFileTitle = ref("CSV Saved");
const savedFileMessage = ref("Your CSV has been saved.");
const isSaving = ref(false);

async function handleDownloadCSV() {
  if (!parseStore.hasRecords || parseStore.isProcessing || isSaving.value) {
    return;
  }

  isSaving.value = true;
  try {
    const res = await parseStore.saveToDownloads();
    if (res && !res.cancelled) {
      savedFileInfo.value = res;
      savedFileTitle.value = "CSV Saved Automatically";
      savedFileMessage.value = "Your CSV has been automatically saved to your Downloads folder.";
      savedFileModalOpen.value = true;
      toast.success(`Saved ${res.filename} to Downloads`, "CSV Exported");
    }
  } catch (err: unknown) {
    toast.error(err, "Failed to Save CSV");
  } finally {
    isSaving.value = false;
  }
}

async function handleSaveAs() {
  if (!parseStore.hasRecords || parseStore.isProcessing || isSaving.value) {
    return;
  }

  isSaving.value = true;
  try {
    const res = await parseStore.saveAs();
    if (res && !res.cancelled) {
      savedFileInfo.value = res;
      savedFileTitle.value = "CSV Saved";
      savedFileMessage.value = "Your CSV has been saved.";
      savedFileModalOpen.value = true;
      toast.success(`Saved ${res.filename}`, "CSV Exported");
    }
  } catch (err: unknown) {
    toast.error(err, "Failed to Save CSV");
  } finally {
    isSaving.value = false;
  }
}

async function handleRevealFile() {
  if (!savedFileInfo.value?.path) {
    return;
  }
  try {
    await parseStore.revealFile(savedFileInfo.value.path);
  } catch (err: unknown) {
    toast.error(err, "Failed to Reveal File");
  }
}
</script>

<template>
  <div class="flex items-center justify-between gap-3 py-2 select-none">
    <!-- Left: Preview Info -->
    <div class="flex items-center gap-2">
      <h3 class="text-sm font-bold text-slate-800 dark:text-slate-100">CSV Preview</h3>
      <span
        v-if="parseStore.hasRecords"
        class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold bg-truck-100 text-truck-800 dark:bg-truck-900/50 dark:text-truck-300 font-mono"
      >
        {{ parseStore.records.length }} records
      </span>
    </div>

    <!-- Right: Export Action Buttons -->
    <div class="flex items-center gap-2">
      <button
        type="button"
        :disabled="!parseStore.hasRecords || parseStore.isProcessing || isSaving"
        class="inline-flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-semibold text-white bg-truck-500 hover:bg-truck-600 active:bg-truck-600 rounded-lg shadow-xs transition-all cursor-pointer focus:outline-hidden focus:ring-2 focus:ring-truck-500 focus:ring-offset-1 disabled:opacity-40 disabled:cursor-not-allowed disabled:shadow-none"
        @click="handleDownloadCSV"
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"
          />
        </svg>
        <span>Download CSV</span>
      </button>

      <button
        type="button"
        :disabled="!parseStore.hasRecords || parseStore.isProcessing || isSaving"
        class="inline-flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 bg-white dark:bg-slate-750 hover:bg-slate-50 dark:hover:bg-slate-700 border border-slate-300 dark:border-slate-650 rounded-lg shadow-xs transition-all cursor-pointer focus:outline-hidden focus:ring-2 focus:ring-truck-500 focus:ring-offset-1 disabled:opacity-40 disabled:cursor-not-allowed disabled:shadow-none"
        @click="handleSaveAs"
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
            d="M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l-3-3m3 3V4"
          />
        </svg>
        <span>Save As…</span>
      </button>
    </div>

    <!-- Saved File Confirmation Modal -->
    <Modal
      :is-open="savedFileModalOpen"
      :title="savedFileTitle"
      max-width-class="max-w-md"
      @close="savedFileModalOpen = false"
    >
      <div class="space-y-4">
        <p class="text-sm text-slate-600 dark:text-slate-300">
          {{ savedFileMessage }}
        </p>

        <div
          class="rounded-lg bg-slate-50 dark:bg-slate-900/80 p-3 border border-slate-200 dark:border-slate-750 space-y-2 text-xs"
        >
          <div>
            <span class="font-semibold text-slate-500 dark:text-slate-400 block mb-0.5"
              >File Name:</span
            >
            <span
              class="font-mono text-slate-800 dark:text-slate-200 select-all break-all font-medium"
              >{{ savedFileInfo?.filename }}</span
            >
          </div>
          <div>
            <span class="font-semibold text-slate-500 dark:text-slate-400 block mb-0.5"
              >Saved To:</span
            >
            <span class="font-mono text-slate-600 dark:text-slate-400 select-all break-all">{{
              savedFileInfo?.dir
            }}</span>
          </div>
        </div>
      </div>

      <template #footer>
        <button
          type="button"
          class="inline-flex items-center gap-1.5 px-3 py-2 text-xs font-semibold text-truck-700 dark:text-truck-300 bg-truck-50 dark:bg-truck-950/60 hover:bg-truck-100 dark:hover:bg-truck-900/60 border border-truck-300 dark:border-truck-800 rounded-lg focus:outline-hidden focus:ring-2 focus:ring-truck-500 cursor-pointer transition-colors"
          @click="handleRevealFile"
        >
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M5 19a2 2 0 01-2-2V7a2 2 0 012-2h4l2 2h4a2 2 0 012 2v1M5 19h14a2 2 0 002-2v-5a2 2 0 00-2-2H9a2 2 0 00-2 2v5a2 2 0 01-2 2z"
            />
          </svg>
          <span>Show in Folder</span>
        </button>

        <button
          type="button"
          class="px-4 py-2 text-xs font-semibold text-white bg-truck-500 hover:bg-truck-600 rounded-lg focus:outline-hidden focus:ring-2 focus:ring-truck-400 transition-colors cursor-pointer"
          @click="savedFileModalOpen = false"
        >
          OK
        </button>
      </template>
    </Modal>
  </div>
</template>
