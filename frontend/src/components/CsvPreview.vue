<script setup lang="ts">
import { useParseStore } from '@/stores/parse'

const parseStore = useParseStore()
interface ColumnDef {
  key: string
  label: string
  minWidth: string
  width: string
  align?: 'left' | 'center' | 'right'
  tabular?: boolean
}

const columns: ColumnDef[] = [
  { key: 'source_file', label: 'source_file', minWidth: '150px', width: '150px' },
  { key: 'subject', label: 'subject', minWidth: '150px', width: '150px' },
  { key: 'message_date', label: 'message_date', minWidth: '120px', width: '120px', tabular: true },
  { key: 'reported_at', label: 'reported_at', minWidth: '120px', width: '120px', tabular: true },
  { key: 'dispatcher', label: 'dispatcher', minWidth: '80px', width: '80px' },
  { key: 'row_in_message', label: 'row_in_message', minWidth: '50px', width: '50px', align: 'center', tabular: true },
  { key: 'raw_entry', label: 'raw_entry', minWidth: '250px', width: '250px' },
  { key: 'location', label: 'location', minWidth: '150px', width: '150px' },
  { key: 'issue', label: 'issue', minWidth: '120px', width: '120px' },
  { key: 'label', label: 'label', minWidth: '80px', width: '80px' },
  { key: 'issue_time', label: 'issue_time', minWidth: '80px', width: '80px', tabular: true },
]
</script>

<template>
  <div class="flex-1 flex flex-col min-h-0 bg-white dark:bg-slate-900 overflow-hidden">
    <!-- Table Container -->
    <div class="flex-1 overflow-auto border border-slate-200 dark:border-slate-800 rounded-lg">
      <table class="w-full text-left text-xs border-collapse">
        <thead class="sticky top-0 z-10 bg-truck-200 dark:bg-slate-800 text-slate-800 dark:text-slate-100 font-bold shadow-xs">
          <tr>
            <th
              v-for="col in columns"
              :key="col.key"
              class="px-3 py-2.5 border-b border-r border-slate-300 dark:border-slate-700 whitespace-nowrap font-bold tracking-tight select-none last:border-r-0"
              :style="{ minWidth: col.minWidth, width: col.width }"
              :class="col.align === 'center' ? 'text-center' : 'text-left'"
            >
              {{ col.label }}
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-200 dark:divide-slate-800 text-slate-700 dark:text-slate-300">
          <tr
            v-for="(rec, idx) in parseStore.records"
            :key="idx"
            class="hover:bg-slate-50 dark:hover:bg-slate-800/60 even:bg-slate-50/40 dark:even:bg-slate-850/40 transition-colors"
          >
            <td class="px-3 py-1.5 border-r border-slate-200 dark:border-slate-800 truncate font-mono text-[11px]" :title="rec.source_file">
              {{ rec.source_file }}
            </td>
            <td class="px-3 py-1.5 border-r border-slate-200 dark:border-slate-800 truncate" :title="rec.subject">
              {{ rec.subject }}
            </td>
            <td class="px-3 py-1.5 border-r border-slate-200 dark:border-slate-800 whitespace-nowrap tabular-nums font-mono text-[11px]">
              {{ rec.message_date }}
            </td>
            <td class="px-3 py-1.5 border-r border-slate-200 dark:border-slate-800 whitespace-nowrap tabular-nums font-mono text-[11px]">
              {{ rec.reported_at }}
            </td>
            <td class="px-3 py-1.5 border-r border-slate-200 dark:border-slate-800 truncate" :title="rec.dispatcher">
              {{ rec.dispatcher }}
            </td>
            <td class="px-3 py-1.5 border-r border-slate-200 dark:border-slate-800 text-center tabular-nums font-mono text-[11px]">
              {{ rec.row_in_message }}
            </td>
            <td class="px-3 py-1.5 border-r border-slate-200 dark:border-slate-800 truncate" :title="rec.raw_entry">
              {{ rec.raw_entry }}
            </td>
            <td class="px-3 py-1.5 border-r border-slate-200 dark:border-slate-800 truncate font-medium" :title="rec.location">
              {{ rec.location }}
            </td>
            <td class="px-3 py-1.5 border-r border-slate-200 dark:border-slate-800 truncate" :title="rec.issue">
              {{ rec.issue }}
            </td>
            <td class="px-3 py-1.5 border-r border-slate-200 dark:border-slate-800 whitespace-nowrap font-mono text-[11px] font-semibold text-truck-600 dark:text-truck-400">
              {{ rec.label }}
            </td>
            <td class="px-3 py-1.5 whitespace-nowrap tabular-nums font-mono text-[11px]">
              {{ rec.issue_time }}
            </td>
          </tr>

          <!-- Empty State Inside Table -->
          <tr v-if="parseStore.records.length === 0">
            <td :colspan="columns.length" class="px-6 py-16 text-center text-slate-400 dark:text-slate-500">
              <div class="flex flex-col items-center justify-center gap-2">
                <svg class="w-8 h-8 text-slate-300 dark:text-slate-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 17v-2m3 2v-4m3 4v-6m2 10H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                </svg>
                <span class="text-sm italic">
                  {{ parseStore.isParsing ? 'Parsing records…' : 'No records parsed yet. Click "Run Parser" to extract rows.' }}
                </span>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
