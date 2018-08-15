Vue.component('signature-pad', {
  template: '#signaturepad',
  props: ['item'],
  delimiters: ['${', '}'],
  data () {
    return {
      signaturePad: null,
      dataUrl: null,
      role: 'Management Company',
      roles: ['Tenant', 'Owner/Landlord', 'Contractor', 'Management Company', 'Agent']
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
    signaturesNeeded: 2,
    html: '',
    pdf: '', // pdf.cool
    ppdf: '', // Prince PDF
    json: '',
    jsonurl: new URL(location.href).searchParams.get('jsonurl') || '/templates/dump.json',
    // all signature urls as example
    signatureDataUris: []
  },
  mounted () {
    fetch(this.jsonurl).then((result) => { return result.text() })
      .then((json) => { this.json = json })
  },
  methods: {
    async submitForm (x) {
      const result = await fetch('/htmlgen', { method: 'POST',
        credentials: 'same-origin',
        headers: { 'X-CSRF-Token': x.target.elements['gorilla.csrf.Token'].value },
        body: new FormData(x.target) })
        .then((result) => { return result.json() })
      this.html = result.HTML
      this.jsonurl = result.JSON

      var params = new URLSearchParams(window.location.search)
      params.set('jsonurl', this.jsonurl)
      const path = window.location.protocol + '//' + window.location.host + window.location.pathname + '?' + params.toString()
      window.history.pushState({path}, '', path)

      fetch(`/pdfgen?url=${result.HTML}`)
        .then(stream => stream.json())
        .then(pdf => this.pdf = pdf.PDF)
      fetch(`/pdfgen?svc=raptor&url=${result.HTML}`)
        .then(stream => stream.json())
        .then(pdf => this.ppdf = pdf.PDF)
    },
    async submitJson (x) {
      // console.log('Submitting JSON', this.json)
      const result = await fetch('/jsonhtmlgen', { method: 'POST',
        credentials: 'same-origin',
        headers: { 'X-CSRF-Token': x.target.elements['gorilla.csrf.Token'].value },
        body: this.json })
        .then((result) => { return result.json() })
      this.html = result.HTML
      this.jsonurl = result.JSON

      var params = new URLSearchParams(window.location.search)
      params.set('jsonurl', this.jsonurl)
      const path = window.location.protocol + '//' + window.location.host + window.location.pathname + '?' + params.toString()
      window.history.pushState({path}, '', path)

      fetch(`/pdfgen?url=${result.HTML}`)
        .then(stream => stream.json())
        .then(pdf => this.pdf = pdf.PDF)
      fetch(`/pdfgen?svc=raptor&url=${result.HTML}`)
        .then(stream => stream.json())
        .then(pdf => this.ppdf = pdf.PDF)
    },
    updateSignature (index, url) {
      Vue.set(this.signatureDataUris, index, url)
      console.log(this.signatureDataUris)
    }
  }
})
