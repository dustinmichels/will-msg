<script setup lang="ts">
import { ref } from 'vue'
import { useRulesStore } from '@/stores/rules'
import { useNavigationStore } from '@/stores/navigation'
import { confirm } from '@/lib/confirm'
import { toast } from '@/lib/toast'
import type { config } from 'wailsjs/go/models'
import RulesTable from '@/components/RulesTable.vue'
import RuleSandbox from '@/components/RuleSandbox.vue'
import RuleFormModal from '@/components/RuleFormModal.vue'

const rulesStore = useRulesStore()
const navigationStore = useNavigationStore()

const isModalOpen = ref(false)
const modalEditIndex = ref(-1)
const modalRule = ref<config.ClassificationRule | null>(null)

function openAddModal() {
  if (rulesStore.isSaving) return
  modalEditIndex.value = -1
  modalRule.value = null
  isModalOpen.value = true
}

function openEditModal(index: number) {
  if (rulesStore.isSaving) return
  if (!rulesStore.workingConfig || index < 0 || index >= rulesStore.workingConfig.rules.length) {
    return
  }
  modalEditIndex.value = index
  modalRule.value = rulesStore.workingConfig.rules[index]
  isModalOpen.value = true
}

async function handleModalSave(payload: {
  rule: Partial<config.ClassificationRule> & { pattern: string; label: string }
  editIndex: number
  customLabel?: config.LabelDefinition
}) {
  try {
    if (payload.customLabel) {
      await rulesStore.addOrUpdateLabel(payload.customLabel)
    }

    if (payload.editIndex >= 0) {
      await rulesStore.updateRule(payload.editIndex, payload.rule)
    } else {
      await rulesStore.addRule(payload.rule)
    }
  } catch (err: unknown) {
    toast.error(err, 'Failed to Save Rule')
  }
}

async function handleDeleteRule() {
  if (rulesStore.isSaving || rulesStore.selectedIndex < 0 || !rulesStore.selectedRule) {
    return
  }

  const idx = rulesStore.selectedIndex
  const rule = rulesStore.selectedRule
  const shouldDelete = await confirm({
    title: 'Delete Rule',
    message: `Are you sure you want to delete rule #${idx + 1} (${rule.pattern})?`,
    confirmText: 'Delete',
    cancelText: 'Cancel',
    danger: true,
  })

  if (shouldDelete) {
    try {
      await rulesStore.deleteRule(idx)
      toast.info(`Deleted rule #${idx + 1}`, 'Rule Deleted')
    } catch (err: unknown) {
      toast.error(err, 'Failed to Delete Rule')
    }
  }
}

async function handleMove(direction: 'up' | 'down') {
  if (rulesStore.isSaving || rulesStore.selectedIndex < 0) return
  try {
    await rulesStore.moveRule(rulesStore.selectedIndex, direction)
  } catch (err: unknown) {
    toast.error(err, 'Failed to Move Rule')
  }
}

async function handleToggle() {
  if (rulesStore.isSaving || rulesStore.selectedIndex < 0) return
  try {
    await rulesStore.toggleRule(rulesStore.selectedIndex)
  } catch (err: unknown) {
    toast.error(err, 'Failed to Toggle Rule')
  }
}

async function handleResetDefaults() {
  if (rulesStore.isSaving) return

  const shouldReset = await confirm({
    title: 'Reset to Built-in Defaults',
    message: 'Are you sure you want to reset all classification rules and labels to built-in defaults?\nAny custom rules will be replaced.',
    confirmText: 'Reset Defaults',
    cancelText: 'Cancel',
    danger: true,
  })

  if (shouldReset) {
    try {
      await rulesStore.resetDefaults()
      toast.info('Classification rules and labels have been reset to built-in defaults.', 'Reset Defaults')
    } catch (err: unknown) {
      toast.error(err, 'Failed to Reset Defaults')
    }
  }
}

async function handleImport() {
  if (rulesStore.isSaving) return
  try {
    const res = await rulesStore.importRulesFromFile()
    if (res && !res.cancelled && res.config) {
      toast.success(`Imported ${res.config.rules?.length || 0} rules successfully.`, 'Rules Imported')
    }
  } catch (err: unknown) {
    toast.error(err, 'Import Failed')
  }
}

async function handleExport() {
  if (rulesStore.isSaving || !rulesStore.workingConfig) return
  try {
    const res = await rulesStore.exportRulesToFile()
    if (res && !res.cancelled) {
      toast.success(`Exported rules to ${res.filename}`, 'Rules Exported')
    }
  } catch (err: unknown) {
    toast.error(err, 'Export Failed')
  }
}

async function handleHeuristicsToggle(e: Event) {
  const checked = (e.target as HTMLInputElement).checked
  try {
    await rulesStore.updateSettings({ enable_heuristics: checked })
  } catch (err: unknown) {
    toast.error(err, 'Failed to Update Settings')
  }
}

async function handleDefaultLabelChange(e: Event) {
  const val = (e.target as HTMLSelectElement).value
  try {
    await rulesStore.updateSettings({ default_label: val })
  } catch (err: unknown) {
    toast.error(err, 'Failed to Update Settings')
  }
}

async function handleSaveAndApply() {
  if (rulesStore.isSaving) return
  try {
    const success = await rulesStore.saveAndApply()
    if (success) {
      toast.success('Classification rules have been saved and applied to the parser.', 'Rules Saved & Applied')
      await navigationStore.closeRules()
    }
  } catch (err: unknown) {
    toast.error(err, 'Save Failed')
  }
}

async function handleCancel() {
  await navigationStore.closeRules()
}
</script>

<template>
  <div class="flex-1 flex flex-col min-h-0 bg-slate-50 dark:bg-slate-900 overflow-hidden select-none">
    <!-- Header Banner -->
    <div class="bg-truck-500 text-white px-4 py-2.5 flex items-center justify-between shadow-xs shrink-0">
      <div class="flex items-center gap-2">
        <svg class="w-5 h-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
        <h2 class="text-base font-bold text-white tracking-tight">
          Rules & Labels Manager
        </h2>
      </div>

      <span class="text-xs italic text-truck-200 hidden sm:inline">
        Customize classification patterns, precedence, labels, and heuristics
      </span>
    </div>

    <!-- Top Controls Section -->
    <div class="p-3 bg-white dark:bg-slate-850 border-b border-slate-200 dark:border-slate-800 shrink-0 space-y-2.5">
      <!-- Toolbar Row -->
      <div class="flex flex-wrap items-center justify-between gap-2">
        <!-- Left Action Buttons -->
        <div class="flex flex-wrap items-center gap-1.5 text-xs">
          <!-- Add Rule -->
          <button
            type="button"
            :disabled="rulesStore.isSaving"
            class="inline-flex items-center gap-1 px-3 py-1.5 font-bold text-white bg-truck-500 hover:bg-truck-600 rounded-lg shadow-xs transition-all cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
            @click="openAddModal"
          >
            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M12 4v16m8-8H4" />
            </svg>
            <span>Add Rule</span>
          </button>

          <!-- Edit -->
          <button
            type="button"
            :disabled="!rulesStore.hasSelection || rulesStore.isSaving"
            class="inline-flex items-center gap-1 px-2.5 py-1.5 font-medium text-slate-700 dark:text-slate-200 bg-slate-100 dark:bg-slate-750 hover:bg-slate-200 dark:hover:bg-slate-700 border border-slate-300 dark:border-slate-650 rounded-lg transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            @click="openEditModal(rulesStore.selectedIndex)"
          >
            <svg class="w-3.5 h-3.5 text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
            </svg>
            <span>Edit</span>
          </button>

          <!-- Delete -->
          <button
            type="button"
            :disabled="!rulesStore.hasSelection || rulesStore.isSaving"
            class="inline-flex items-center gap-1 px-2.5 py-1.5 font-medium text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-950/30 hover:bg-red-100 dark:hover:bg-red-900/50 border border-red-200 dark:border-red-900/60 rounded-lg transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            @click="handleDeleteRule"
          >
            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
            <span>Delete</span>
          </button>

          <span class="h-4 w-px bg-slate-300 dark:bg-slate-700 mx-1"></span>

          <!-- Move Up -->
          <button
            type="button"
            :disabled="!rulesStore.canMoveUp || rulesStore.isSaving"
            class="inline-flex items-center gap-1 px-2.5 py-1.5 font-medium text-slate-700 dark:text-slate-200 bg-slate-100 dark:bg-slate-750 hover:bg-slate-200 dark:hover:bg-slate-700 border border-slate-300 dark:border-slate-650 rounded-lg transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            @click="handleMove('up')"
          >
            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 15l7-7 7 7" />
            </svg>
            <span>Move Up</span>
          </button>

          <!-- Move Down -->
          <button
            type="button"
            :disabled="!rulesStore.canMoveDown || rulesStore.isSaving"
            class="inline-flex items-center gap-1 px-2.5 py-1.5 font-medium text-slate-700 dark:text-slate-200 bg-slate-100 dark:bg-slate-750 hover:bg-slate-200 dark:hover:bg-slate-700 border border-slate-300 dark:border-slate-650 rounded-lg transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            @click="handleMove('down')"
          >
            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
            </svg>
            <span>Move Down</span>
          </button>

          <!-- Toggle On/Off -->
          <button
            type="button"
            :disabled="!rulesStore.hasSelection || rulesStore.isSaving"
            class="inline-flex items-center gap-1 px-2.5 py-1.5 font-medium text-slate-700 dark:text-slate-200 bg-slate-100 dark:bg-slate-750 hover:bg-slate-200 dark:hover:bg-slate-700 border border-slate-300 dark:border-slate-650 rounded-lg transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            @click="handleToggle"
          >
            <svg class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            <span>Toggle On/Off</span>
          </button>
        </div>

        <!-- Right Action Buttons -->
        <div class="flex items-center gap-1.5 text-xs">
          <!-- Reset Defaults -->
          <button
            type="button"
            :disabled="rulesStore.isSaving"
            class="inline-flex items-center gap-1 px-2.5 py-1.5 font-medium text-slate-700 dark:text-slate-200 bg-white dark:bg-slate-750 hover:bg-slate-100 dark:hover:bg-slate-700 border border-slate-300 dark:border-slate-650 rounded-lg transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            @click="handleResetDefaults"
          >
            <svg class="w-3.5 h-3.5 text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            <span>Reset Defaults</span>
          </button>

          <!-- Import JSON -->
          <button
            type="button"
            :disabled="rulesStore.isSaving"
            class="inline-flex items-center gap-1 px-2.5 py-1.5 font-medium text-slate-700 dark:text-slate-200 bg-white dark:bg-slate-750 hover:bg-slate-100 dark:hover:bg-slate-700 border border-slate-300 dark:border-slate-650 rounded-lg transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            @click="handleImport"
          >
            <svg class="w-3.5 h-3.5 text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
            </svg>
            <span>Import JSON</span>
          </button>

          <!-- Export JSON -->
          <button
            type="button"
            :disabled="rulesStore.isSaving"
            class="inline-flex items-center gap-1 px-2.5 py-1.5 font-medium text-slate-700 dark:text-slate-200 bg-white dark:bg-slate-750 hover:bg-slate-100 dark:hover:bg-slate-700 border border-slate-300 dark:border-slate-650 rounded-lg transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            @click="handleExport"
          >
            <svg class="w-3.5 h-3.5 text-slate-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
            <span>Export JSON</span>
          </button>
        </div>
      </div>

      <!-- Settings Bar Row -->
      <div class="flex flex-wrap items-center justify-between gap-3 pt-2 border-t border-slate-100 dark:border-slate-800 text-xs">
        <div class="flex items-center gap-2">
          <input
            id="heuristics-check"
            type="checkbox"
            :checked="rulesStore.workingConfig?.enable_heuristics ?? true"
            :disabled="rulesStore.isSaving"
            class="w-4 h-4 rounded text-truck-500 focus:ring-truck-500 border-slate-300 dark:border-slate-600 cursor-pointer disabled:opacity-50"
            @change="handleHeuristicsToggle"
          />
          <label for="heuristics-check" class="text-slate-700 dark:text-slate-300 font-medium cursor-pointer">
            Enable heuristic fallback matching (e.g. *_not_out)
          </label>
        </div>

        <div class="flex items-center gap-4">
          <div class="flex items-center gap-2">
            <span class="text-slate-600 dark:text-slate-400">Fallback Default Label:</span>
            <select
              :value="rulesStore.workingConfig?.default_label"
              :disabled="rulesStore.isSaving"
              class="px-2 py-1 text-xs rounded border border-slate-300 dark:border-slate-650 bg-white dark:bg-slate-750 text-slate-900 dark:text-slate-100 focus:ring-2 focus:ring-truck-500 cursor-pointer disabled:opacity-50 font-mono"
              @change="handleDefaultLabelChange"
            >
              <option
                v-for="l in rulesStore.workingConfig?.labels || []"
                :key="l.key"
                :value="l.key"
              >
                {{ l.key }}
              </option>
            </select>
          </div>

          <div class="font-bold text-slate-800 dark:text-slate-200">
            Total Rules: {{ rulesStore.rulesCount }}
          </div>
        </div>
      </div>
    </div>

    <!-- Main Content Area: Rules Table + Sandbox -->
    <div class="flex-1 min-h-0 flex flex-col p-3 gap-3 overflow-y-auto">
      <!-- Rules Table Section (min 200px, flexible) -->
      <div class="flex-1 min-h-[200px] flex flex-col">
        <RulesTable @edit="openEditModal" />
      </div>

      <!-- Live Sandbox Panel -->
      <div class="shrink-0">
        <RuleSandbox />
      </div>
    </div>

    <!-- Bottom Action Bar -->
    <div class="px-4 py-3 bg-white dark:bg-slate-850 border-t border-slate-200 dark:border-slate-800 flex items-center justify-between shrink-0 shadow-xs">
      <div class="flex items-center gap-3">
        <span class="text-xs text-slate-500 dark:text-slate-400 italic">
          Changes take effect immediately upon saving.
        </span>
        <span
          v-if="rulesStore.isDirty"
          class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] font-semibold bg-amber-100 dark:bg-amber-950/60 text-amber-800 dark:text-amber-300 border border-amber-300 dark:border-amber-800"
        >
          <span class="w-1.5 h-1.5 rounded-full bg-amber-500 animate-pulse"></span>
          <span>Unsaved changes</span>
        </span>
      </div>

      <div class="flex items-center gap-2.5">
        <!-- Cancel Button -->
        <button
          type="button"
          :disabled="rulesStore.isSaving"
          class="px-4 py-1.5 text-xs font-semibold text-slate-700 dark:text-slate-200 bg-white dark:bg-slate-750 hover:bg-slate-100 dark:hover:bg-slate-700 border border-slate-300 dark:border-slate-650 rounded-lg transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
          @click="handleCancel"
        >
          Cancel
        </button>

        <!-- Save & Apply Rules Button -->
        <button
          type="button"
          :disabled="rulesStore.isSaving"
          class="inline-flex items-center gap-1.5 px-4 py-1.5 text-xs font-semibold text-white bg-truck-500 hover:bg-truck-600 active:bg-truck-600 rounded-lg shadow-sm transition-all cursor-pointer focus:outline-hidden focus:ring-2 focus:ring-truck-500 focus:ring-offset-1 disabled:opacity-50 disabled:cursor-not-allowed"
          @click="handleSaveAndApply"
        >
          <svg v-if="rulesStore.isSaving" class="animate-spin h-3.5 w-3.5 text-white" fill="none" viewBox="0 0 24 24">
            <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
            <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
          </svg>
          <svg v-else class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l-3-3m3 3V4" />
          </svg>
          <span>{{ rulesStore.isSaving ? 'Saving…' : 'Save & Apply Rules' }}</span>
        </button>
      </div>
    </div>

    <!-- Rule Form Modal -->
    <RuleFormModal
      :is-open="isModalOpen"
      :rule="modalRule"
      :edit-index="modalEditIndex"
      :labels="rulesStore.workingConfig?.labels || []"
      @close="isModalOpen = false"
      @save="handleModalSave"
    />
  </div>
</template>
