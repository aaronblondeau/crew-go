import { defineStore } from 'pinia'
import { api } from 'boot/axios'

export interface Task {
  id: string,
  taskGroupId: string
  name: string
  worker: string
  workgroup: string
  key: string
  remainingAttempts: number
  isPaused: boolean
  isComplete: boolean
  runAfter: string
  isSeed: boolean
  errorDelayInSeconds: number
  input: any
  output: any
  errors: Array<any>
  createdAt: string
  parentIds: any // Array<string>
  busyExecuting: boolean
  pauseWait: boolean
  resumeWait: boolean
  retryWait: boolean
  resetWait: boolean
}

export interface ModifyTask {
  id?: string
  name: string
  worker: string
  workgroup?: string
  key?: string
  remainingAttempts?: number
  isPaused?: boolean
  isComplete?: boolean
  runAfter?: string
  isSeed?: boolean
  errorDelayInSeconds?: number
  input?: any
  parentIds?: Array<string>
}

export interface PaginatedTasks {
  tasks: Array<Task>,
  count: number
}

export const useTaskStore = defineStore('task', {
  actions: {
    async getTasks (taskGroupId: string, page = 1, pageSize = 20, search = '') : Promise<PaginatedTasks> {
      const result = await api.get(`api/v1/task_group/${taskGroupId}/tasks`, {
        params: {
          page,
          pageSize,
          search
        }
      })
      return result.data
    },
    async getTask (taskGroupId: string, taskId: string) : Promise<Task> {
      const result = await api.get(`api/v1/task_group/${taskGroupId}/task/${taskId}`)
      return result.data
    },
    async updateTask (taskGroupId: string, taskId: string, payload: {name: string}) : Promise<Task> {
      const result = await api.put(`api/v1/task_group/${taskGroupId}/task/${taskId}`, payload)
      return result.data
    },
    async createTask (taskGroupId: string, payload: ModifyTask) : Promise<Task> {
      const result = await api.post(`api/v1/task_group/${taskGroupId}/tasks`, payload)
      return result.data
    },
    async deleteTask (taskGroupId: string, taskId: string) {
      await api.delete(`api/v1/task_group/${taskGroupId}/task/${taskId}`)
    },
    async pauseTask (taskGroupId: string, taskId: string) {
      const result = await api.put(`api/v1/task_group/${taskGroupId}/task/${taskId}`, { isPaused: true })
      return result.data
    },
    async resumeTask (taskGroupId: string, taskId: string) {
      const result = await api.put(`api/v1/task_group/${taskGroupId}/task/${taskId}`, { isPaused: false })
      return result.data
    },
    async resetTask (taskGroupId: string, taskId: string, remainingAttempts = 5) {
      const result = await api.post(`api/v1/task_group/${taskGroupId}/task/${taskId}/reset`, { remainingAttempts })
      return result.data
    },
    async retryTask (taskGroupId: string, taskId: string, remainingAttempts = 5) {
      const result = await api.post(`api/v1/task_group/${taskGroupId}/task/${taskId}/retry`, { remainingAttempts })
      return result.data
    }
  }
})
