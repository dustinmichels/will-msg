<script setup lang="ts">
import { useNavigationStore } from "@/stores/navigation";
import { useFileDrop } from "@/composables/useFileDrop";

import AppHeader from "@/components/AppHeader.vue";
import ToastHost from "@/components/ToastHost.vue";
import ConfirmDialog from "@/components/ConfirmDialog.vue";

import WelcomeScreen from "@/components/WelcomeScreen.vue";
import SourcePanel from "@/components/SourcePanel.vue";
import ExportBar from "@/components/ExportBar.vue";
import SkippedSourcesNotice from "@/components/SkippedSourcesNotice.vue";
import CsvPreview from "@/components/CsvPreview.vue";
import RulesView from "@/components/RulesView.vue";

const navigationStore = useNavigationStore();

// Register drag-and-drop listener once at top level
useFileDrop();
</script>

<template>
  <div
    id="app-root"
    class="h-screen w-screen flex flex-col bg-slate-50 text-slate-900 dark:bg-slate-900 dark:text-slate-50 overflow-hidden select-none"
  >
    <!-- Top App Header -->
    <AppHeader />

    <!-- Main Dynamic Content Body -->
    <main class="flex-1 min-h-0 flex flex-col overflow-hidden relative">
      <!-- 1. Rules View -->
      <RulesView v-if="navigationStore.view === 'rules'" />

      <!-- 2. Workspace View -->
      <div
        v-else-if="navigationStore.view === 'workspace'"
        class="flex-1 flex min-h-0 overflow-hidden"
      >
        <!-- Left Source Panel (approx 35% or 320px) -->
        <div class="w-80 md:w-96 shrink-0 h-full flex flex-col">
          <SourcePanel />
        </div>

        <!-- Right CSV Preview Panel -->
        <div
          class="flex-1 h-full min-w-0 flex flex-col p-3 bg-white dark:bg-slate-900 overflow-hidden"
        >
          <!-- Top Export Action Bar -->
          <ExportBar />

          <!-- Skipped Files Notice Banner (if any) -->
          <SkippedSourcesNotice />

          <!-- Main Scrollable CSV Table -->
          <CsvPreview />
        </div>
      </div>

      <!-- 3. Welcome View -->
      <WelcomeScreen v-else />
    </main>

    <!-- Global Toast & Confirmation Dialog Hosts -->
    <ToastHost />
    <ConfirmDialog />
  </div>
</template>

<style scoped></style>
