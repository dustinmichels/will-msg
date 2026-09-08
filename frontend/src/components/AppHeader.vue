<script setup lang="ts">
import { useNavigationStore } from "@/stores/navigation";
import truckImg from "@/assets/truck.png";

const navigationStore = useNavigationStore();

async function handleRulesClick() {
  if (navigationStore.isRulesOpen) {
    await navigationStore.closeRules();
  } else {
    await navigationStore.openRules();
  }
}
</script>

<template>
  <header
    class="h-[50px] bg-truck-500 text-white px-4 flex items-center justify-between shadow-xs select-none shrink-0"
  >
    <div class="flex items-center space-x-3">
      <img :src="truckImg" alt="Truck Logo" class="w-10 h-10 object-contain drop-shadow-xs" />
      <h1 class="text-title font-bold text-white tracking-tight">Outlook MSG Parser</h1>
    </div>

    <div class="flex items-center space-x-3">
      <button
        type="button"
        class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold tracking-wide transition-all shadow-xs cursor-pointer focus:outline-hidden focus:ring-2 focus:ring-white/60"
        :class="
          navigationStore.isRulesOpen
            ? 'bg-white text-truck-600 shadow-sm'
            : 'bg-white/20 text-white hover:bg-white/30 border border-white/25'
        "
        @click="handleRulesClick"
      >
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
          />
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"
          />
        </svg>
        <span>{{ navigationStore.isRulesOpen ? "Back to Parser" : "Rules & Labels" }}</span>
      </button>

      <span class="text-subtitle italic text-truck-200 hidden md:inline select-none pl-2">
        Feed me your msg files, Will
      </span>
    </div>
  </header>
</template>
