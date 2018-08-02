Vue.component('signature-pad', {
  template: '#signaturepad',
  props: ['item'],
  delimiters: ['${', '}'],
  data () {
    return {
      signaturePad: null,
      dataUrl: null
    }
  },
  mounted () {
    this.signaturePad = new SignaturePad(this.$refs.canv, {
      onEnd: () => {
        this.dataUrl = this.signaturePad.toDataURL()
        // as example collect all url in parent
        this.$emit('update', this.dataUrl)
      },
      backgroundColor: 'rgb(255, 255, 255)'
    })
  }
})

new Vue({
  el: '#app',
  delimiters: ['${', '}'],
  data: {
    signaturesNeeded: 1,
    html: '',
    // all signature urls as example
    signatureDataUris: []
  },
  methods: {
    async submitForm (x) {
      const result = await fetch('/pdfgen', { method: 'POST',
        credentials: 'same-origin',
        headers: { 'X-CSRF-Token': x.target.elements['gorilla.csrf.Token'].value },
        body: new FormData(x.target) })
        .then((result) => { return result.json() })
      this.html = result.HTML
    },
    updateSignature (index, url) {
      Vue.set(this.signatureDataUris, index, url)
      console.log(this.signatureDataUris)
    }
  }
})
