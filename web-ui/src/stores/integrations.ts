import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Integration } from '@/types'
import { api, type IntegrationInput } from '@/api'

export type { IntegrationInput }

/** Notification integrations (channels) + CRUD, backed by the REST API. */
export const useIntegrationsStore = defineStore('integrations', () => {
  const integrations = ref<Integration[]>([])
  const loading = ref(false)
  const loaded = ref(false)

  function replace(it: Integration) {
    const i = integrations.value.findIndex((x) => x.id === it.id)
    if (i >= 0) integrations.value[i] = it
  }

  async function fetchIntegrations() {
    loading.value = true
    try {
      integrations.value = await api.listIntegrations()
      loaded.value = true
    } finally {
      loading.value = false
    }
  }

  async function create(input: IntegrationInput): Promise<Integration> {
    const it = await api.createIntegration(input)
    integrations.value.push(it)
    return it
  }

  async function update(id: number, input: IntegrationInput): Promise<Integration> {
    const it = await api.updateIntegration(id, input)
    replace(it)
    return it
  }

  async function remove(id: number) {
    await api.deleteIntegration(id)
    integrations.value = integrations.value.filter((x) => x.id !== id)
  }

  function test(id: number) {
    return api.testIntegration(id)
  }

  return { integrations, loading, loaded, fetchIntegrations, create, update, remove, test }
})
