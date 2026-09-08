import { ref } from 'vue'

export type ToastType = 'info' | 'success' | 'warning' | 'error'

export interface ToastItem {
  id: string
  message: string
  title?: string
  type: ToastType
  duration: number
  timerId?: number
}

export const toasts = ref<ToastItem[]>([])

let nextId = 1

export function addToast(
  message: string,
  type: ToastType = 'info',
  title?: string,
  duration = 4000
): string {
  const id = `toast-${nextId++}-${Date.now()}`

  const item: ToastItem = {
    id,
    message,
    title,
    type,
    duration,
  }

  if (duration > 0) {
    const timer = setTimeout(() => {
      removeToast(id)
    }, duration)
    item.timerId = timer as unknown as number
  }

  toasts.value.push(item)
  return id
}

export function removeToast(id: string): void {
  const index = toasts.value.findIndex(t => t.id === id)
  if (index !== -1) {
    const item = toasts.value[index]
    if (item.timerId !== undefined) {
      clearTimeout(item.timerId)
    }
    toasts.value.splice(index, 1)
  }
}

export function clearToasts(): void {
  for (const item of toasts.value) {
    if (item.timerId !== undefined) {
      clearTimeout(item.timerId)
    }
  }
  toasts.value = []
}

export const toast = {
  info(message: string, title?: string, duration = 4000): string {
    return addToast(message, 'info', title, duration)
  },
  success(message: string, title?: string, duration = 4000): string {
    return addToast(message, 'success', title, duration)
  },
  warning(message: string, title?: string, duration = 5000): string {
    return addToast(message, 'warning', title, duration)
  },
  error(err: string | Error | unknown, title?: string, duration = 6000): string {
    const message = err instanceof Error ? err.message : String(err)
    return addToast(message, 'error', title, duration)
  },
  remove: removeToast,
  clear: clearToasts,
  toasts,
}
