import axios from 'axios'

const API_BASE_URL = import.meta.env.VITE_SERVER_URL_V1

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json'
  }
})
