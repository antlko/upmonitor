import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Incident, IncidentDetail } from '@/types'
import { api, type IncidentInput } from '@/api'

export type { IncidentInput }

/** Rows per page on the incidents table. */
export const INCIDENTS_PAGE_SIZE = 20

/** Incidents list + CRUD, backed by the REST API. */
export const useIncidentsStore = defineStore('incidents', () => {
  const incidents = ref<Incident[]>([])
  const total = ref(0)
  const page = ref(1)
  const loading = ref(false)
  const loaded = ref(false)

  function replace(inc: Incident) {
    const i = incidents.value.findIndex((x) => x.id === inc.id)
    if (i >= 0) incidents.value[i] = inc
  }

  /** page defaults to the currently loaded page (1 on first load). */
  async function fetchIncidents(params: { status?: string; serviceId?: string; page?: number } = {}) {
    const targetPage = params.page ?? page.value
    loading.value = true
    try {
      const res = await api.listIncidents({
        status: params.status,
        serviceId: params.serviceId,
        limit: INCIDENTS_PAGE_SIZE,
        offset: (targetPage - 1) * INCIDENTS_PAGE_SIZE,
      })
      incidents.value = res.incidents
      total.value = res.total
      page.value = targetPage
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
    total.value += 1
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
    total.value = Math.max(0, total.value - 1)
  }

  return {
    incidents,
    total,
    page,
    loading,
    loaded,
    fetchIncidents,
    getDetail,
    create,
    update,
    resolve,
    remove,
  }
})
