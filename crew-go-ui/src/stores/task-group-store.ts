import { defineStore } from 'pinia'
import { api } from 'boot/axios'

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
        }
      })
      return result.data
    },
    async updateTaskGroup (id: string, payload: {name: string}) : Promise<TaskGroup> {
      const result = await api.put(`api/v1/task_group/${id}`, payload)
      return result.data
    },
    async createTaskGroup (id: string, name: string) : Promise<TaskGroup> {
      const result = await api.post('api/v1/task_groups', {
        id,
        name
      })
      return result.data
    }
  }
})
