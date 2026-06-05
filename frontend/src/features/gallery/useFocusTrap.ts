import { useEffect, useRef } from 'react'

// Selector for elements that can receive keyboard focus inside the trap.
const FOCUSABLE_SELECTOR = [
  'button:not([disabled])',
  '[href]',
  'input:not([disabled])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(', ')

/**
 * useFocusTrap keeps Tab / Shift+Tab focus cycling within a container while
 * `active` is true. Returns a ref to attach to the container element.
 *
 * The dialog sets initial focus and restores it on close separately; this hook
 * only handles wrap-around so keyboard users cannot tab out into the page
 * behind a modal (the missing half of `aria-modal="true"`).
 */
export function useFocusTrap<T extends HTMLElement>(active: boolean) {
  const ref = useRef<T>(null)

  useEffect(() => {
    if (!active) return
    const node = ref.current
    if (!node) return

    function handleKeyDown(e: KeyboardEvent) {
      if (e.key !== 'Tab' || !node) return
      // The selector already excludes disabled controls and tabindex="-1".
      const focusables = Array.from(
        node.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR),
      )
      const first = focusables[0]
      const last = focusables[focusables.length - 1]
      if (!first || !last) {
        e.preventDefault()
        return
      }
      const activeEl = document.activeElement

      if (e.shiftKey) {
        if (activeEl === first || !node.contains(activeEl)) {
          e.preventDefault()
          last.focus()
        }
      } else if (activeEl === last || !node.contains(activeEl)) {
        e.preventDefault()
        first.focus()
      }
    }

    node.addEventListener('keydown', handleKeyDown)
    return () => node.removeEventListener('keydown', handleKeyDown)
  }, [active])

  return ref
}
