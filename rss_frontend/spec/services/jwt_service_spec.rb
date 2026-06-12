require 'rails_helper'

RSpec.describe JwtService, type: :service do
  fixtures :users

  describe '.generate_token' do
    let(:user) { users(:alice) }

    it 'generates a signed ES256 token and updates the user jwt_jti' do
      expect(user.jwt_jti).to be_nil

      token = JwtService.generate_token(user_id: user.id)

      expect(token).to be_present

      # Verify that user record got updated with a unique uuid jti
      user.reload
      expect(user.jwt_jti).to be_present

      # Decode token payload
      private_key_path = Rails.root.join("keys", "ec_private.pem")
      private_key_pem = File.read(private_key_path)
      private_key = OpenSSL::PKey::EC.new(private_key_pem)

      decoded = JWT.decode(token, private_key, true, { algorithm: 'ES256' })
      payload = decoded.first

      expect(payload['sub']).to eq(user.id)
      expect(payload['jti']).to eq(user.jwt_jti)
      expect(payload['exp']).to be_present
    end
  end
end
