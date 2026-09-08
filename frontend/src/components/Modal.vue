<script setup lang="ts">
import { onMounted, onUnmounted } from "vue";

const props = withDefaults(
  defineProps<{
    isOpen: boolean;
    title?: string;
    maxWidthClass?: string;
  }>(),
  {
    isOpen: false,
    title: "",
    maxWidthClass: "max-w-lg",
  },
);

const emit = defineEmits<{
  (e: "close"): void;
}>();

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Escape" && props.isOpen) {
    emit("close");
  }
}

onMounted(() => {
  window.addEventListener("keydown", onKeydown);
});

onUnmounted(() => {
  window.removeEventListener("keydown", onKeydown);
});
</script>

<template>
  <Teleport to="body">
    <div
      v-if="isOpen"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs transition-opacity duration-150"
      @click.self="emit('close')"
      role="dialog"
      aria-modal="true"
    >
      <div
        class="w-full rounded-xl bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 shadow-2xl border border-slate-200 dark:border-slate-700 flex flex-col max-h-[90vh] overflow-hidden transform transition-transform"
        :class="maxWidthClass"
      >
        <!-- Header -->
        <div
          v-if="title || $slots.header"
          class="flex items-center justify-between px-6 py-4 border-b border-slate-200 dark:border-slate-700"
        >
          <slot name="header">
            <h3 class="text-lg font-semibold tracking-tight">{{ title }}</h3>
          </slot>
          <button
            type="button"
            class="p-1 rounded-lg text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-700 focus:outline-hidden focus:ring-2 focus:ring-truck-500 transition-colors"
            @click="emit('close')"
            aria-label="Close dialog"
          >
            <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M6 18L18 6M6 6l12 12"
              />
            </svg>
          </button>
        </div>

        <!-- Body -->
        <div class="px-6 py-4 overflow-y-auto flex-1">
          <slot></slot>
        </div>

        <!-- Footer -->
        <div
          v-if="$slots.footer"
          class="px-6 py-3.5 bg-slate-50 dark:bg-slate-800/80 border-t border-slate-200 dark:border-slate-700 flex items-center justify-end gap-3"
        >
          <slot name="footer"></slot>
        </div>
      </div>
    </div>
  </Teleport>
</template>
