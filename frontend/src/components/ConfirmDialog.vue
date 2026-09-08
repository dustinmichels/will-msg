<script setup lang="ts">
import { activeConfirm, resolveConfirm } from "@/lib/confirm";
import Modal from "@/components/Modal.vue";

function onConfirm() {
  resolveConfirm(true);
}

function onCancel() {
  resolveConfirm(false);
}
</script>

<template>
  <Modal
    :is-open="activeConfirm !== null"
    :title="activeConfirm?.title || 'Confirm'"
    max-width-class="max-w-md"
    @close="onCancel"
  >
    <div
      class="py-2 text-sm text-slate-600 dark:text-slate-300 leading-relaxed whitespace-pre-line"
    >
      {{ activeConfirm?.message }}
    </div>

    <template #footer>
      <button
        type="button"
        class="px-4 py-2 text-sm font-medium text-slate-700 dark:text-slate-200 bg-white dark:bg-slate-700 border border-slate-300 dark:border-slate-600 rounded-lg hover:bg-slate-50 dark:hover:bg-slate-600 focus:outline-hidden focus:ring-2 focus:ring-slate-400 transition-colors cursor-pointer"
        @click="onCancel"
      >
        {{ activeConfirm?.cancelText || "Cancel" }}
      </button>
      <button
        type="button"
        class="px-4 py-2 text-sm font-medium text-white rounded-lg focus:outline-hidden focus:ring-2 transition-colors cursor-pointer"
        :class="
          activeConfirm?.danger
            ? 'bg-red-600 hover:bg-red-700 focus:ring-red-400'
            : 'bg-truck-500 hover:bg-truck-600 focus:ring-truck-400'
        "
        @click="onConfirm"
      >
        {{ activeConfirm?.confirmText || "Confirm" }}
      </button>
    </template>
  </Modal>
</template>
