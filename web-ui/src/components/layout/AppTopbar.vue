<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { LogOut, Settings, UserRound, ChevronDown } from '@lucide/vue'
import ThemeToggle from '@/components/common/ThemeToggle.vue'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Separator } from '@/components/ui/separator'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
} from '@/components/ui/dropdown-menu'
import { Badge } from '@/components/ui/badge'
import { useAuthStore } from '@/stores/auth'
import { initials } from '@/lib/format'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const title = computed(() => (route.meta.title as string) ?? 'upmonitor')
const user = computed(() => auth.currentUser)

async function signOut() {
  await auth.logout()
  router.push('/login')
}
</script>

<template>
  <header
    class="flex h-14 shrink-0 items-center gap-3 border-b border-border bg-background/80 px-5 backdrop-blur-xl"
  >
    <h1 class="text-sm font-semibold tracking-tight">{{ title }}</h1>

    <div class="ml-auto flex items-center gap-2">
      <!-- Live auto-refresh indicator -->
      <div
        class="hidden items-center gap-2 rounded-full border border-border bg-card px-3 py-1.5 sm:flex"
      >
        <span class="relative flex size-2">
          <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-online opacity-75" />
          <span class="relative inline-flex size-2 rounded-full bg-online" />
        </span>
        <span class="text-xs font-medium text-muted-foreground">Live</span>
      </div>

      <ThemeToggle />

      <Separator orientation="vertical" class="mx-0.5 h-6" />

      <!-- User menu -->
      <DropdownMenu>
        <DropdownMenuTrigger
          class="flex items-center gap-2 rounded-lg p-1 pr-2 outline-none transition-colors hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring/40"
        >
          <Avatar class="size-7">
            <AvatarFallback class="bg-primary/15 text-xs text-primary">
              {{ user ? initials(user.username) : '?' }}
            </AvatarFallback>
          </Avatar>
          <span class="hidden text-sm font-medium sm:block">{{ user?.username }}</span>
          <ChevronDown class="hidden size-3.5 text-muted-foreground sm:block" />
        </DropdownMenuTrigger>
        <DropdownMenuContent class="w-56">
          <DropdownMenuLabel>
            <div class="flex flex-col gap-1">
              <span class="text-sm font-medium text-foreground">{{ user?.username }}</span>
              <Badge :variant="user?.role === 'admin' ? 'default' : 'secondary'" class="w-fit">
                {{ user?.role === 'admin' ? 'Administrator' : 'Read only' }}
              </Badge>
            </div>
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuItem @select="router.push('/settings')">
            <UserRound />
            Account
          </DropdownMenuItem>
          <DropdownMenuItem @select="router.push('/settings')">
            <Settings />
            Settings
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem variant="destructive" @select="signOut">
            <LogOut />
            Sign out
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  </header>
</template>
