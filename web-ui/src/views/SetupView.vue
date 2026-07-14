<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ShieldCheck, Loader2, Check, X } from '@lucide/vue'
import BrandMark from '@/components/common/BrandMark.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { toast } from '@/components/ui/sonner'
import { useAuthStore } from '@/stores/auth'
import { ApiError } from '@/api'
import { cn } from '@/lib/utils'

const router = useRouter()
const auth = useAuthStore()

const username = ref('admin')
const password = ref('')
const confirm = ref('')
const loading = ref(false)

const longEnough = computed(() => password.value.length >= 8)
const matches = computed(() => confirm.value.length > 0 && password.value === confirm.value)
const valid = computed(() => username.value.trim().length > 0 && longEnough.value && matches.value)

async function submit() {
  if (!valid.value || loading.value) return
  loading.value = true
  try {
    await auth.setup(username.value.trim(), password.value)
    router.push('/')
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : 'Could not create account')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="relative flex min-h-screen items-center justify-center overflow-hidden bg-background px-6">
    <div
      class="pointer-events-none absolute left-1/2 top-0 h-[420px] w-[720px] -translate-x-1/2 rounded-full bg-primary/15 blur-[120px]"
    />
    <div class="relative w-full max-w-sm">
      <div class="mb-7 flex flex-col items-center text-center">
        <BrandMark class="size-11" />
        <h1 class="mt-4 text-xl font-semibold tracking-tight">Welcome to upmonitor</h1>
        <p class="mt-1.5 text-sm text-muted-foreground">
          Create your administrator account to get started.
        </p>
      </div>

      <div class="rounded-2xl border border-border bg-card p-6 shadow-elevation-medium">
        <form class="grid gap-4" @submit.prevent="submit">
          <div class="grid gap-2">
            <Label for="username">Username</Label>
            <Input id="username" v-model="username" autocomplete="username" />
          </div>
          <div class="grid gap-2">
            <Label for="password">Password</Label>
            <Input
              id="password"
              v-model="password"
              type="password"
              autocomplete="new-password"
              placeholder="At least 8 characters"
            />
          </div>
          <div class="grid gap-2">
            <Label for="confirm">Confirm password</Label>
            <Input id="confirm" v-model="confirm" type="password" autocomplete="new-password" />
          </div>

          <ul class="grid gap-1.5 text-xs">
            <li :class="cn('flex items-center gap-1.5', longEnough ? 'text-online' : 'text-muted-foreground')">
              <Check v-if="longEnough" class="size-3.5" />
              <X v-else class="size-3.5" />
              At least 8 characters
            </li>
            <li :class="cn('flex items-center gap-1.5', matches ? 'text-online' : 'text-muted-foreground')">
              <Check v-if="matches" class="size-3.5" />
              <X v-else class="size-3.5" />
              Passwords match
            </li>
          </ul>

          <Button type="submit" class="mt-1 w-full" :disabled="!valid || loading">
            <Loader2 v-if="loading" class="animate-spin" />
            <ShieldCheck v-else />
            Create account
          </Button>
        </form>
      </div>

      <p class="mt-5 text-center text-xs text-muted-foreground">
        Your password is hashed with bcrypt and stored locally. It never leaves your server.
      </p>
    </div>
  </div>
</template>
