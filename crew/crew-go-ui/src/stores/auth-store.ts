import { defineStore } from 'pinia'
import { api } from 'boot/axios'

export const useAuthStore = defineStore('auth', {
  state: () => {
    // regex for ?token= in url
    const tokenRegex = /token=([^&]+)/
    const injectedToken = tokenRegex.exec(window.location.href)?.[1] || ''
    if (injectedToken) {
      localStorage.setItem('token', injectedToken)
    }
    return {
      // Look for token in local storage, url (iframe embeds)
      token: localStorage.getItem('token') || '',
      authenticated: false
    }
  },
  actions: {
    async login (username: string, password: string) {
      const result = await api.post('login', {
        username,
        password
      })
      this.token = result.data.token
      localStorage.setItem('token', this.token)
      this.authenticated = true
    },
    logout () {
      this.token = ''
      localStorage.removeItem('token')
      this.authenticated = false
    },
    async checkAuth () {
      try {
        const result = await api.get('authcheck', {
          headers: {
            Authorization: `Bearer ${this.token}`
          }
        })
        if (result.status === 200) {
          this.authenticated = true
        } else {
          this.authenticated = false
        }
      } catch (e) {
        this.authenticated = false
      }
    }
  }
})
