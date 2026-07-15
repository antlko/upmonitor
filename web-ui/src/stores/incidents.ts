import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Incident, IncidentDetail } from '@/types'
import { api, type IncidentInput } from '@/api'

export type { IncidentInput }

/** Incidents list + CRUD, backed by the REST API. */
export const useIncidentsStore = defineStore('incidents', () => {
  const incidents = ref<Incident[]>([])
  const loading = ref(false)
  const loaded = ref(false)

  function replace(inc: Incident) {
    const i = incidents.value.findIndex((x) => x.id === inc.id)
    if (i >= 0) incidents.value[i] = inc
  }

  async function fetchIncidents(params: { status?: string; serviceId?: string } = {}) {
    loading.value = true
    try {
      incidents.value = await api.listIncidents(params)
      loaded.value = true
    } finally {
      loading.value = false
    }
  }

  function getDetail(id: number): Promise<IncidentDetail> {
    return api.getIncident(id)
  }

  async function create(input: IncidentInput): Promise<Incident> {
    const inc = await api.createIncident(input)
    incidents.value.unshift(inc)
    return inc
  }

  async function update(id: number, input: IncidentInput): Promise<Incident> {
    const inc = await api.updateIncident(id, input)
    replace(inc)
    return inc
  }

  function resolve(id: number): Promise<Incident> {
    return update(id, { resolvedAt: new Date().toISOString() })
  }

  async function remove(id: number) {
    await api.deleteIncident(id)
    incidents.value = incidents.value.filter((i) => i.id !== id)
  }

  return { incidents, loading, loaded, fetchIncidents, getDetail, create, update, resolve, remove }
})
