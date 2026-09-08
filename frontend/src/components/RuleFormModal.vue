<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import type { config } from 'wailsjs/go/models'
import Modal from '@/components/Modal.vue'

const props = defineProps<{
  isOpen: boolean
  rule: config.ClassificationRule | null
  editIndex: number
  labels: config.LabelDefinition[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (
    e: 'save',
    payload: {
      rule: Partial<config.ClassificationRule> & { pattern: string; label: string }
      editIndex: number
      customLabel?: config.LabelDefinition
    }
  ): void
}>()

const isNew = computed(() => props.editIndex < 0 || !props.rule)
const modalTitle = computed(() => (isNew.value ? 'Add Classification Rule' : `Edit Rule #${props.editIndex + 1}`))

const pattern = ref('')
const matchType = ref<'substring' | 'regex'>('substring')
const targetLabel = ref('')
const description = ref('')
const enabled = ref(true)

// Custom Label Definition
const isAddingCustomLabel = ref(false)
const customKey = ref('')
const customDisplayName = ref('')
const customMetric = ref<'none' | 'trash' | 'recycling' | 'both'>('none')

const errorMessage = ref<string | null>(null)

watch(
  () => props.isOpen,
  (open) => {
    if (open) {
      errorMessage.value = null
      isAddingCustomLabel.value = false
      customKey.value = ''
      customDisplayName.value = ''
      customMetric.value = 'none'

      if (props.rule && props.editIndex >= 0) {
        pattern.value = props.rule.pattern
        matchType.value = (props.rule.type as 'substring' | 'regex') || 'substring'
        targetLabel.value = props.rule.label
        description.value = props.rule.description || ''
        enabled.value = props.rule.enabled
      } else {
        pattern.value = ''
        matchType.value = 'substring'
        targetLabel.value = props.labels.length > 0 ? props.labels[0].key : ''
        description.value = ''
        enabled.value = true
      }
    }
  },
  { immediate: true }
)

function onTargetLabelChange(val: string) {
  if (val === '__ADD_NEW__') {
    isAddingCustomLabel.value = true
    targetLabel.value = '__ADD_NEW__'
  } else {
    isAddingCustomLabel.value = false
    targetLabel.value = val
  }
}

function handleSave() {
  errorMessage.value = null
  let pat = pattern.value.trim()

  if (!pat) {
    errorMessage.value = 'Rule pattern cannot be empty'
    return
  }

  if (matchType.value === 'substring') {
    pat = pat.toUpperCase()
  } else if (matchType.value === 'regex') {
    try {
      new RegExp(pat)
    } catch (e: unknown) {
      errorMessage.value = `Invalid regex pattern: ${e instanceof Error ? e.message : String(e)}`
      return
    }
  }

  let finalLabel = targetLabel.value
  let customDef: config.LabelDefinition | undefined

  if (isAddingCustomLabel.value || targetLabel.value === '__ADD_NEW__') {
    const key = customKey.value.trim()
    if (!key) {
      errorMessage.value = 'New label key cannot be empty'
      return
    }
    const displayName = customDisplayName.value.trim() || key
    customDef = {
      key,
      display_name: displayName,
      metric: customMetric.value,
    } as config.LabelDefinition
    finalLabel = key
  }

  if (!finalLabel) {
    errorMessage.value = 'Target label must be selected'
    return
  }

  emit('save', {
    rule: {
      id: props.rule?.id,
      pattern: pat,
      type: matchType.value,
      label: finalLabel,
      description: description.value.trim(),
      enabled: enabled.value,
    },
    editIndex: props.editIndex,
    customLabel: customDef,
  })

  emit('close')
}
</script>

<template>
  <Modal
    :is-open="isOpen"
    :title="modalTitle"
    max-width-class="max-w-lg"
    @close="emit('close')"
  >
    <form @submit.prevent="handleSave" class="space-y-4 text-xs select-none">
      <!-- Error Alert -->
      <div v-if="errorMessage" class="p-2.5 rounded-lg bg-red-50 dark:bg-red-950/60 border border-red-300 dark:border-red-800 text-red-800 dark:text-red-200">
        {{ errorMessage }}
      </div>

      <!-- Pattern -->
      <div>
        <label class="block font-bold text-slate-700 dark:text-slate-300 mb-1">
          Pattern: <span class="text-red-500">*</span>
        </label>
        <input
          v-model="pattern"
          type="text"
          placeholder="e.g. MSW AND RECYC NOT OUT or \bBLOCKED\b"
          class="w-full px-3 py-2 text-xs font-mono rounded-lg border border-slate-300 dark:border-slate-600 bg-truck-input/40 dark:bg-slate-700 text-slate-900 dark:text-slate-100 focus:outline-hidden focus:ring-2 focus:ring-truck-500"
          required
        />
      </div>

      <!-- Match Type -->
      <div>
        <label class="block font-bold text-slate-700 dark:text-slate-300 mb-1">
          Match Type:
        </label>
        <select
          v-model="matchType"
          class="w-full px-3 py-2 text-xs rounded-lg border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-700 text-slate-900 dark:text-slate-100 focus:outline-hidden focus:ring-2 focus:ring-truck-500 cursor-pointer"
        >
          <option value="substring">substring</option>
          <option value="regex">regex</option>
        </select>
      </div>

      <!-- Target Label -->
      <div>
        <label class="block font-bold text-slate-700 dark:text-slate-300 mb-1">
          Target Label: <span class="text-red-500">*</span>
        </label>
        <select
          :value="isAddingCustomLabel ? '__ADD_NEW__' : targetLabel"
          class="w-full px-3 py-2 text-xs rounded-lg border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-700 text-slate-900 dark:text-slate-100 focus:outline-hidden focus:ring-2 focus:ring-truck-500 cursor-pointer"
          @change="onTargetLabelChange(($event.target as HTMLSelectElement).value)"
        >
          <option v-for="l in labels" :key="l.key" :value="l.key">
            {{ l.key }} ({{ l.display_name }})
          </option>
          <option value="__ADD_NEW__">
            [+ Add New Label…]
          </option>
        </select>
      </div>

      <!-- New Label Definition Box -->
      <div
        v-if="isAddingCustomLabel"
        class="p-3.5 rounded-lg border border-truck-300 dark:border-truck-800 bg-truck-50/60 dark:bg-truck-950/30 space-y-3"
      >
        <h4 class="font-bold text-truck-800 dark:text-truck-300 text-xs">
          New Label Definition:
        </h4>

        <div>
          <label class="block text-slate-600 dark:text-slate-400 mb-1">
            Label Key: <span class="text-red-500">*</span>
          </label>
          <input
            v-model="customKey"
            type="text"
            placeholder="e.g. yard_waste_not_out"
            class="w-full px-2.5 py-1.5 text-xs font-mono rounded border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-700 text-slate-900 dark:text-slate-100 focus:outline-hidden focus:ring-2 focus:ring-truck-500"
          />
        </div>

        <div>
          <label class="block text-slate-600 dark:text-slate-400 mb-1">
            Display Name:
          </label>
          <input
            v-model="customDisplayName"
            type="text"
            placeholder="e.g. Yard Waste Not Out"
            class="w-full px-2.5 py-1.5 text-xs rounded border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-700 text-slate-900 dark:text-slate-100 focus:outline-hidden focus:ring-2 focus:ring-truck-500"
          />
        </div>

        <div>
          <label class="block text-slate-600 dark:text-slate-400 mb-1">
            Metric Contribution:
          </label>
          <select
            v-model="customMetric"
            class="w-full px-2.5 py-1.5 text-xs rounded border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-700 text-slate-900 dark:text-slate-100 focus:outline-hidden focus:ring-2 focus:ring-truck-500 cursor-pointer"
          >
            <option value="none">none</option>
            <option value="trash">trash</option>
            <option value="recycling">recycling</option>
            <option value="both">both</option>
          </select>
        </div>
      </div>

      <!-- Description -->
      <div>
        <label class="block font-bold text-slate-700 dark:text-slate-300 mb-1">
          Description:
        </label>
        <input
          v-model="description"
          type="text"
          placeholder="Optional rule notes"
          class="w-full px-3 py-2 text-xs rounded-lg border border-slate-300 dark:border-slate-600 bg-white dark:bg-slate-700 text-slate-900 dark:text-slate-100 focus:outline-hidden focus:ring-2 focus:ring-truck-500"
        />
      </div>

      <!-- Enabled Toggle -->
      <div class="flex items-center gap-2 pt-1">
        <input
          id="rule-enabled-checkbox"
          v-model="enabled"
          type="checkbox"
          class="w-4 h-4 rounded text-truck-500 focus:ring-truck-500 border-slate-300 dark:border-slate-600 cursor-pointer"
        />
        <label for="rule-enabled-checkbox" class="font-bold text-slate-700 dark:text-slate-300 cursor-pointer">
          Rule Enabled
        </label>
      </div>
    </form>

    <template #footer>
      <button
        type="button"
        class="px-4 py-2 text-xs font-semibold text-slate-700 dark:text-slate-200 bg-white dark:bg-slate-700 border border-slate-300 dark:border-slate-600 rounded-lg hover:bg-slate-50 dark:hover:bg-slate-600 transition-colors cursor-pointer"
        @click="emit('close')"
      >
        Cancel
      </button>

      <button
        type="button"
        class="px-4 py-2 text-xs font-semibold text-white bg-truck-500 hover:bg-truck-600 rounded-lg focus:outline-hidden focus:ring-2 focus:ring-truck-400 transition-colors cursor-pointer shadow-xs"
        @click="handleSave"
      >
        Save Rule
      </button>
    </template>
  </Modal>
</template>
