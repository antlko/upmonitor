import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/** Merge class names, resolving Tailwind conflicts (used by every UI component). */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
