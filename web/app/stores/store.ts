import { defineStore } from 'pinia'
import type { Artwork } from '~/types/artwork'

export const usePiniaStore = defineStore('ManyACG', {
  state: () => ({
    preferLight: false,
    r18: false
  }),
  actions: {
    setpreferLight(preferLight: boolean) {
      this.preferLight = preferLight
    },
    setR18(r18: boolean) {
      this.r18 = r18
    }
  },
  persist: true
})

export const useArtworkStore = defineStore('Artwork', {
  state: () => ({
    artworks: {} as Record<string, Artwork>,
    maxCacheSize: 50
  }),
  actions: {
    addArtwork(artwork: Artwork) {
      const id = artwork.id
      this.artworks[id] = artwork
    },
    getArtwork(id: string) {
      const artwork = this.artworks[id]
      if (Object.keys(this.artworks).length > this.maxCacheSize) {
        this.clearArtworks()
      }
      return artwork || null
    },
    clearArtworks() {
      this.artworks = {}
    },
    removeArtwork(id: string) {
      if (!this.artworks[id]) return
      delete this.artworks[id]
    }
  }
})
