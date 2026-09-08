<script setup lang="ts">
import { useParseStore } from '@/stores/parse'
import { toast } from '@/lib/toast'

const parseStore = useParseStore()

async function handleRunParser() {
  if (!parseStore.hasSources || parseStore.isProcessing) {
    return
  }

  try {
    const res = await parseStore.parse()
    if (!res) {
      return
    }

    if (res.superseded) {
      return
    }

    if (res.records.length === 0) {
      toast.info('No structured records found in selected files.', 'No Data')
    } else {
      if (res.skipped && res.skipped.length > 0) {
        toast.warning(`Parsed ${res.records.length} records. ${res.skipped.length} source file(s) were skipped due to errors.`, 'Parse Completed with Warnings')
      } else {
        toast.success(`Successfully parsed ${res.records.length} records.`, 'Parse Completed')
      }
    }
  } catch (err: unknown) {
    toast.error(err, 'Parsing Error')
  }
}
</script>

<template>
  <button
    type="button"
    :disabled="!parseStore.hasSources || parseStore.isProcessing"
    class="w-full inline-flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl font-bold text-sm text-white shadow-md hover:shadow-lg transition-all cursor-pointer focus:outline-hidden focus:ring-2 focus:ring-truck-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed disabled:shadow-none"
    :class="parseStore.isParsing
      ? 'bg-truck-600'
      : 'bg-truck-500 hover:bg-truck-600 active:bg-truck-600'"
    @click="handleRunParser"
  >
    <svg v-if="parseStore.isParsing" class="animate-spin -ml-1 mr-2 h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
    </svg>

    <svg v-else class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>

    <span>{{ parseStore.isParsing ? 'Parsing Messages…' : 'Run Parser' }}</span>
  </button>
</template>
