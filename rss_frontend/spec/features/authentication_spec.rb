require 'rails_helper'

RSpec.describe "Authentication", type: :feature, js: true do
  fixtures :users

  scenario "user can log in with valid credentials" do
    visit "/"
    expect(page).to have_current_path("/login")

    fill_in "Email",    with: users(:alice).email
    fill_in "Password", with: "password"
    click_button "Sign in"

    expect(page).to have_current_path("/feeds")
    expect(page).to have_text("Signed in successfully")
  end

  scenario "user sees error with invalid credentials" do
    visit "/login"
    fill_in "Email",    with: "wrong@example.com"
    fill_in "Password", with: "wrong"
    click_button "Sign in"

    expect(page).to have_text("Invalid email or password")
    expect(page).to have_current_path("/login")
  end

  scenario "user can log out" do
    sign_in_as users(:alice)
    click_button "Sign out"

    expect(page).to have_current_path("/login")
  end

  scenario "unauthenticated user is redirected to login" do
    visit "/feeds"
    expect(page).to have_current_path("/login")
  end
end
