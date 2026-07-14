<script setup lang="ts">
import { onMounted, ref } from 'vue'
import {
  Globe,
  Users,
  FolderCog,
  Upload,
  Download,
  Sun,
  Moon,
  Trash2,
  UserPlus,
  LayoutGrid,
} from '@lucide/vue'
import PageHeader from '@/components/layout/PageHeader.vue'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from '@/components/ui/select'
import { toast } from '@/components/ui/sonner'
import { useSettingsStore } from '@/stores/settings'
import { useUiStore } from '@/stores/ui'
import { useAuthStore } from '@/stores/auth'
import { api, ApiError } from '@/api'
import type { UserRole, WidgetMode } from '@/types'
import { initials } from '@/lib/format'
import { cn } from '@/lib/utils'

const settings = useSettingsStore()
const ui = useUiStore()
const auth = useAuthStore()

const configPath = ref('')
const importInput = ref<HTMLInputElement>()
const newUsername = ref('')
const newPassword = ref('')
const newRole = ref<UserRole>('readonly')
const busy = ref(false)

const themeOptions = [
  { value: 'light' as const, icon: Sun, label: 'Light' },
  { value: 'dark' as const, icon: Moon, label: 'Dark' },
]

onMounted(async () => {
  try {
    await Promise.all([settings.fetch(), auth.fetchUsers()])
    configPath.value = settings.settings.configDir
  } catch (e) {
    toast.error(errMsg(e))
  }
})

function errMsg(e: unknown) {
  return e instanceof ApiError ? e.message : 'Something went wrong'
}

async function setPublic(value: boolean) {
  try {
    await settings.update({ publicDashboard: value })
    toast.success(value ? 'Public dashboard enabled' : 'Public dashboard disabled')
  } catch (e) {
    toast.error(errMsg(e))
  }
}
async function setDefaultMode(value: WidgetMode) {
  try {
    await settings.update({ defaultWidgetMode: value })
  } catch (e) {
    toast.error(errMsg(e))
  }
}
async function applyConfigPath() {
  const path = configPath.value.trim()
  if (!path) return
  busy.value = true
  try {
    await settings.setConfigPath(path)
    toast.success('Config folder updated — reloading…')
    setTimeout(() => window.location.reload(), 600)
  } catch (e) {
    toast.error(errMsg(e))
    busy.value = false
  }
}
async function exportConfig() {
  try {
    const blob = await api.exportConfig()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'upmonitor-backup.zip'
    a.click()
    URL.revokeObjectURL(url)
  } catch (e) {
    toast.error(errMsg(e))
  }
}
async function onImport(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  busy.value = true
  try {
    const res = await api.importConfig(file)
    toast.success(`Imported ${res.services} service(s) — reloading…`)
    setTimeout(() => window.location.reload(), 700)
  } catch (err) {
    toast.error(errMsg(err))
    busy.value = false
  }
}
async function addUser() {
  const name = newUsername.value.trim()
  if (!name || newPassword.value.length < 8) return
  try {
    await auth.addUser(name, newPassword.value, newRole.value)
    toast.success(`Invited ${name}`)
    newUsername.value = ''
    newPassword.value = ''
    newRole.value = 'readonly'
  } catch (e) {
    toast.error(errMsg(e))
  }
}
async function removeUser(id: number, name: string) {
  try {
    await auth.removeUser(id)
    toast.success(`Removed ${name}`)
  } catch (e) {
    toast.error(errMsg(e))
  }
}
</script>

<template>
  <div class="mx-auto max-w-3xl px-6 py-6 lg:px-8">
    <PageHeader title="Settings" description="Access, users and configuration." />

    <Tabs default-value="general" class="mt-6">
      <TabsList class="mb-6">
        <TabsTrigger value="general"><Globe /> General</TabsTrigger>
        <TabsTrigger value="users"><Users /> Users</TabsTrigger>
        <TabsTrigger value="config"><FolderCog /> Configuration</TabsTrigger>
      </TabsList>

      <!-- General -->
      <TabsContent value="general" class="grid gap-3">
        <div class="flex items-center justify-between gap-4 rounded-xl border border-border bg-card p-4">
          <div class="min-w-0">
            <p class="text-sm font-medium">Public dashboard</p>
            <p class="mt-0.5 text-xs text-muted-foreground">
              Allow anyone to view a read-only dashboard at
              <code class="rounded bg-muted px-1 py-0.5 text-[11px]">/public</code> without signing in.
            </p>
          </div>
          <Switch
            :model-value="settings.settings.publicDashboard"
            @update:model-value="setPublic"
          />
        </div>

        <div class="flex items-center justify-between gap-4 rounded-xl border border-border bg-card p-4">
          <div class="min-w-0">
            <p class="text-sm font-medium">Appearance</p>
            <p class="mt-0.5 text-xs text-muted-foreground">Choose your interface theme.</p>
          </div>
          <div class="inline-flex shrink-0 rounded-lg bg-muted p-1">
            <button
              v-for="opt in themeOptions"
              :key="opt.value"
              type="button"
              :class="
                cn(
                  'flex items-center gap-1.5 rounded-md px-3 py-1.5 text-sm font-medium transition-all',
                  ui.theme === opt.value
                    ? 'bg-card text-foreground shadow-elevation-low'
                    : 'text-muted-foreground hover:text-foreground',
                )
              "
              @click="ui.theme = opt.value"
            >
              <component :is="opt.icon" class="size-4" />
              {{ opt.label }}
            </button>
          </div>
        </div>

        <div class="flex items-center justify-between gap-4 rounded-xl border border-border bg-card p-4">
          <div class="min-w-0">
            <p class="text-sm font-medium">Default widget mode</p>
            <p class="mt-0.5 text-xs text-muted-foreground">The layout used for new services.</p>
          </div>
          <Select :model-value="settings.settings.defaultWidgetMode" @update:model-value="(v) => setDefaultMode(v as WidgetMode)">
            <SelectTrigger class="w-44">
              <LayoutGrid class="size-4 text-muted-foreground" />
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="icon">Icon only</SelectItem>
              <SelectItem value="name">Icon + name</SelectItem>
              <SelectItem value="dashboard">Mini dashboard</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </TabsContent>

      <!-- Users -->
      <TabsContent value="users" class="grid gap-3">
        <div class="overflow-hidden rounded-xl border border-border bg-card">
          <div
            v-for="u in auth.users"
            :key="u.id"
            class="flex items-center gap-3 border-b border-border/60 p-4 last:border-0"
          >
            <Avatar class="size-9">
              <AvatarFallback class="bg-primary/15 text-primary">{{ initials(u.username) }}</AvatarFallback>
            </Avatar>
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <p class="truncate text-sm font-medium">{{ u.username }}</p>
                <Badge :variant="u.role === 'admin' ? 'default' : 'secondary'">
                  {{ u.role === 'admin' ? 'Admin' : 'Read only' }}
                </Badge>
                <Badge v-if="u.id === auth.currentUser?.id" variant="outline">You</Badge>
              </div>
            </div>
            <Button
              v-if="u.id !== auth.currentUser?.id"
              variant="ghost"
              size="icon-sm"
              class="text-muted-foreground hover:text-offline"
              aria-label="Remove user"
              @click="removeUser(u.id, u.username)"
            >
              <Trash2 />
            </Button>
          </div>
        </div>

        <div class="rounded-xl border border-border bg-card p-4">
          <p class="text-sm font-medium">Invite someone</p>
          <p class="mt-0.5 text-xs text-muted-foreground">
            Create an account for a friend (password must be at least 8 characters).
          </p>
          <div class="mt-3 grid gap-2 sm:grid-cols-[1fr_1fr_auto]">
            <Input v-model="newUsername" placeholder="username" autocomplete="off" />
            <Input v-model="newPassword" type="password" placeholder="password" autocomplete="new-password" />
            <Select v-model="newRole">
              <SelectTrigger class="sm:w-36">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="readonly">Read only</SelectItem>
                <SelectItem value="admin">Admin</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="mt-2 flex justify-end">
            <Button :disabled="!newUsername.trim() || newPassword.length < 8" @click="addUser">
              <UserPlus />
              Create account
            </Button>
          </div>
        </div>
      </TabsContent>

      <!-- Configuration -->
      <TabsContent value="config" class="grid gap-3">
        <div class="rounded-xl border border-border bg-card p-4">
          <p class="text-sm font-medium">Config folder</p>
          <p class="mt-0.5 text-xs text-muted-foreground">
            Where config.yaml, images and the database live. Changing this reloads the app.
          </p>
          <div class="mt-3 flex flex-col gap-2 sm:flex-row">
            <Input v-model="configPath" placeholder="/config" class="flex-1 font-mono text-xs" />
            <Button variant="outline" :disabled="busy" @click="applyConfigPath">Apply</Button>
          </div>
        </div>

        <div class="flex flex-wrap items-center justify-between gap-4 rounded-xl border border-border bg-card p-4">
          <div class="min-w-0">
            <p class="text-sm font-medium">Backup &amp; restore</p>
            <p class="mt-0.5 text-xs text-muted-foreground">
              Export or import a .zip with config.yaml and images. Import backs up the current config
              first.
            </p>
          </div>
          <div class="flex gap-2">
            <Button variant="outline" :disabled="busy" @click="importInput?.click()">
              <Upload />
              Import
            </Button>
            <Button variant="outline" @click="exportConfig">
              <Download />
              Export
            </Button>
          </div>
        </div>

        <div class="rounded-xl border border-border bg-card p-4">
          <div class="flex items-center justify-between">
            <div>
              <p class="text-sm font-medium">Metrics retention</p>
              <p class="mt-0.5 text-xs text-muted-foreground">
                History older than this is deleted automatically.
              </p>
            </div>
            <Badge variant="secondary">{{ settings.settings.check.retentionDays }} days</Badge>
          </div>
        </div>

        <input ref="importInput" type="file" accept=".zip" class="hidden" @change="onImport" />
      </TabsContent>
    </Tabs>
  </div>
</template>
