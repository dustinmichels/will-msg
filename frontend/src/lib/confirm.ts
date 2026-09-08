import { ref } from 'vue'

export interface ConfirmOptions {
  title?: string
  message: string
  confirmText?: string
  cancelText?: string
  danger?: boolean
}

export interface ConfirmRequest extends ConfirmOptions {
  id: string
  resolve: (value: boolean) => void
}

export const activeConfirm = ref<ConfirmRequest | null>(null)

let nextConfirmId = 1

export function confirm(options: ConfirmOptions): Promise<boolean> {
  return new Promise<boolean>((resolve) => {
    activeConfirm.value = {
      ...options,
      id: `confirm-${nextConfirmId++}-${Date.now()}`,
      resolve: (result: boolean) => {
        activeConfirm.value = null
        resolve(result)
      },
    }
  })
}

export function resolveConfirm(result: boolean): void {
  if (activeConfirm.value) {
    activeConfirm.value.resolve(result)
  }
}

export const confirmDialog = {
  confirm,
  resolve: resolveConfirm,
  activeConfirm,
}
