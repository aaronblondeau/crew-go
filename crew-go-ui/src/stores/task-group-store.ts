import { defineStore } from 'pinia'
import { api } from 'boot/axios'
import { useAuthStore } from './auth-store'

const authStore = useAuthStore()

export interface TaskGroup {
  id: string
  name: string
  createdAt: string
}

export interface PaginatedTaskGroups {
  taskGroups: Array<TaskGroup>,
  count: number
}

export const useTaskGroupStore = defineStore('taskGroup', {
//   state: () => ({
//     counter: 0
//   }),
//   getters: {
//     doubleCount: (state) => state.counter * 2
//   },
  actions: {
    async getTaskGroups (page = 1, pageSize = 20, search = '') : Promise<PaginatedTaskGroups> {
      const result = await api.get('api/v1/task_groups', {
        params: {
          page,
          pageSize,
          search
        },
        headers: {
          Authorization: `Bearer ${authStore.token}`
        }
      })
      return result.data
    },
    async getTaskGroup (id: string) : Promise<TaskGroup> {
      const result = await api.get(`api/v1/task_group/${id}`, {
        headers: {
          Authorization: `Bearer ${authStore.token}`
        }
      })
      return result.data
    },
    async updateTaskGroup (id: string, payload: {name: string}) : Promise<TaskGroup> {
      const result = await api.put(`api/v1/task_group/${id}`, payload, {
        headers: {
          Authorization: `Bearer ${authStore.token}`
        }
      })
      return result.data
    },
    async createTaskGroup (id: string, name: string) : Promise<TaskGroup> {
      const result = await api.post('api/v1/task_groups', {
        id,
        name
      }, {
        headers: {
          Authorization: `Bearer ${authStore.token}`
        }
      })
      return result.data
    },
    async deleteTaskGroup (id: string) {
      await api.delete(`api/v1/task_group/${id}`, {
        headers: {
          Authorization: `Bearer ${authStore.token}`
        }
      })
    },
    async resetTaskGroup (id: string, remainingAttempts = 5) : Promise<TaskGroup> {
      const result = await api.post(`api/v1/task_group/${id}/reset`, { remainingAttempts }, {
        headers: {
          Authorization: `Bearer ${authStore.token}`
        }
      })
      return result.data
    },
    async retryTaskGroup (id: string, remainingAttempts = 5) : Promise<TaskGroup> {
      const result = await api.post(`api/v1/task_group/${id}/retry`, { remainingAttempts }, {
        headers: {
          Authorization: `Bearer ${authStore.token}`
        }
      })
      return result.data
    },
    async pauseTaskGroup (id: string) : Promise<TaskGroup> {
      const result = await api.post(`api/v1/task_group/${id}/pause`, {}, {
        headers: {
          Authorization: `Bearer ${authStore.token}`
        }
      })
      return result.data
    },
    async resumeTaskGroup (id: string) : Promise<TaskGroup> {
      const result = await api.post(`api/v1/task_group/${id}/resume`, {}, {
        headers: {
          Authorization: `Bearer ${authStore.token}`
        }
      })
      return result.data
    },
    async getTaskGroupProgress (id: string) : Promise<number> {
      const result = await api.get(`api/v1/task_group/${id}/progress`, {
        headers: {
          Authorization: `Bearer ${authStore.token}`
        }
      })
      return result.data.completedPercent
    },
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    async watchTaskGroup (id: string, onEvent: (evt: any) => void) : Promise<() => void> {
      const socket = new WebSocket(`ws://localhost:8090/api/v1/task_group/${id}/stream/${authStore.token}`)

      socket.onopen = function () {
        console.log('~~ connected to task group stream')
      }

      socket.onmessage = function (event) {
        onEvent(event.data)
      }

      socket.onclose = function () {
        console.log('~~ closed task group stream')
      }

      return () => {
        console.log('~~ closing task group stream')
        socket.close()
      }
    }
  }
})
