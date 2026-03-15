import axios from 'axios'
export const api = axios.create({
  baseURL: import.meta.env.VITE_APP_ENV == 'dev' ? 'http://localhost:8080/api' : '/api',
  withCredentials: true,
  headers: { 'Content-Type': 'application/json' },
})
